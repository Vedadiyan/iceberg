package netio

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type (
	Cloner   func(options ...RequestOption) (*http.Request, error)
	Response struct {
		*http.Response
		error
	}
	Caller interface {
		GetIsParallel() bool
		GetName() string
		GetAwaitList() []string
		Call(context.Context, Cloner) (bool, *http.Response, error)
		GetRequestUpdaters() []RequestUpdater
		GetResponseUpdaters() []ResponseUpdater
		GetContext() context.Context
	}
)

func Cascade(in *ShadowRequest, callers ...Caller) (*ShadowResponse, error) {
	var (
		out *ShadowResponse
		mut sync.RWMutex
	)
	in.Header.Add("X-Request-Id", uuid.New().String())
	tasks := make(map[string]<-chan *Response)
	ctx := make(map[string]context.Context)
	for _, cal := range callers {
		err := await(cal, &mut, ctx, tasks, in, out)
		if err != nil {
			return nil, err
		}
		if cal.GetIsParallel() {
			spin(cal, &mut, ctx, tasks, in)
			continue
		}
		term, res, err := cal.Call(cal.GetContext(), in.CloneRequest)
		if err != nil {
			return nil, err
		}
		if term {
			return NewShandowResponse(res)
		}
		out, err = createOrUpdateResponse(out, res, append(cal.GetResponseUpdaters(), ResUpdateHeader("X-Request-Id")))
		if err != nil {
			return nil, err
		}
		tmp, err := out.CreateRequest()
		if err != nil {
			return nil, err
		}

		err = UpdateRequest(in, tmp.Request, append(cal.GetRequestUpdaters(), ReqUpdateHeader("X-Request-Id")))
		if err != nil {
			return nil, err
		}
		in.Reset()
	}
	out.Reset()
	return out, nil
}

func await(cal Caller, mut *sync.RWMutex, ctxs map[string]context.Context, tsks map[string]<-chan *Response, in *ShadowRequest, out *ShadowResponse) error {
	if len(cal.GetAwaitList()) != 0 {
		for _, task := range cal.GetAwaitList() {
			var err error
			mut.RLocker()
			ch, chFound := tsks[task]
			ctx, ctxFound := ctxs[task]
			mut.RUnlock()
			if !chFound || !ctxFound {
				return fmt.Errorf("task not found")
			}
			var cr *Response
			select {
			case cr = <-ch:
				{
					break
				}
			case <-ctx.Done():
				{
					cr = &Response{
						error: context.DeadlineExceeded,
					}
				}
			}
			if cr.error != nil {
				return cr.error
			}
			out, err = createOrUpdateResponse(out, cr.Response, cal.GetResponseUpdaters())
			if err != nil {
				return err
			}
			tmp, err := out.CreateRequest()
			if err != nil {
				return err
			}

			err = UpdateRequest(in, tmp.Request, cal.GetRequestUpdaters())
			if err != nil {
				return err
			}
			in.Reset()
		}
	}
	return nil
}

func spin(cal Caller, mut *sync.RWMutex, ctxs map[string]context.Context, tsks map[string]<-chan *Response, in *ShadowRequest) {
	go func() {
		ch := make(chan *Response, 1)
		mut.Lock()
		tsks[cal.GetName()] = ch
		ctxs[cal.GetName()] = cal.GetContext()
		mut.Unlock()
		_, r, err := cal.Call(cal.GetContext(), in.CloneRequest)
		if err != nil {
			ch <- &Response{
				error: err,
			}
			return
		}
		ch <- &Response{
			Response: r,
		}
	}()
}

func createOrUpdateResponse(in *ShadowResponse, res *http.Response, ru []ResponseUpdater) (*ShadowResponse, error) {
	if in == nil {
		return NewShandowResponse(res)

	}
	err := UpdateResponse(in, res, ru)
	if err != nil {
		return nil, err
	}
	in.Reset()
	return in, nil
}
