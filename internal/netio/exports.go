package netio

import (
	"context"
	"fmt"
	"net/http"
	"sync"
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
		Call(context.Context, Cloner) (*http.Response, error)
		GetRequestUpdaters() []RequestUpdater
		GetResponseUpdaters() []ResponseUpdater
		GetContext() context.Context
	}
)

func Cascade(i *ShadowRequest, callers ...Caller) (*ShadowResponse, error) {
	var (
		o   *ShadowResponse
		mut sync.RWMutex
	)
	tasks := make(map[string]<-chan *Response)
	ctx := make(map[string]context.Context)
	for _, c := range callers {
		err := await(c, &mut, ctx, tasks, i, o)
		if err != nil {
			return nil, err
		}
		if c.GetIsParallel() {
			spin(c, &mut, ctx, tasks, i)
			continue
		}
		r, err := c.Call(c.GetContext(), i.CloneRequest)
		if err != nil {
			return nil, err
		}
		o, err = createOrUpdateResponse(o, r, c.GetResponseUpdaters())
		if err != nil {
			return nil, err
		}
		tmp, err := o.CreateRequest()
		if err != nil {
			return nil, err
		}

		err = UpdateRequest(i, tmp.Request, c.GetRequestUpdaters())
		if err != nil {
			return nil, err
		}
		i.Reset()
	}
	o.Reset()
	return o, nil
}

func await(cal Caller, mute *sync.RWMutex, ctxs map[string]context.Context, tsks map[string]<-chan *Response, in *ShadowRequest, out *ShadowResponse) error {
	if len(cal.GetAwaitList()) != 0 {
		for _, task := range cal.GetAwaitList() {
			var err error
			mute.RLocker()
			ch, chFound := tsks[task]
			ctx, ctxFound := ctxs[task]
			mute.RUnlock()
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
		r, err := cal.Call(cal.GetContext(), in.CloneRequest)
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

func createOrUpdateResponse(shadowResponse *ShadowResponse, response *http.Response, updaters []ResponseUpdater) (*ShadowResponse, error) {
	if shadowResponse == nil {
		return NewShandowResponse(response)

	}
	err := UpdateResponse(shadowResponse, response, updaters)
	if err != nil {
		return nil, err
	}
	shadowResponse.Reset()
	return shadowResponse, nil
}
