package netio

import (
	"context"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type (
	Next     bool
	Level    int
	Cloner   func(options ...RequestOption) (*http.Request, error)
	Response struct {
		*http.Response
		Error
	}
	Error interface {
		Message() string
		Status() int
	}
	Caller interface {
		GetLevel() Level
		GetIsParallel() bool
		GetName() string
		GetAwaitList() []string
		Call(context.Context, RouteValues, Cloner, Cloner) (Next, *http.Response, Error)
		GetRequestUpdaters() []RequestUpdater
		GetResponseUpdaters() []ResponseUpdater
		GetContext() context.Context
	}
	httpError struct {
		message string
		status  int
	}
)

const (
	TERM     Next = true
	CONTINUE Next = false

	LEVEL_NONE     Level = 1
	LEVEL_CONNECT  Level = 2
	LEVEL_PRE      Level = 4
	LEVEL_REQUEST  Level = 8
	LEVEL_RESPONSE Level = 16
	LEVEL_POST     Level = 32
)

func NewError(message string, status int) Error {
	return &httpError{
		message: message,
		status:  status,
	}
}

func Sort(callers ...Caller) []Caller {
	main := make([]Caller, 0)
	connect := make([]Caller, 0)
	pre := make([]Caller, 0)
	request := make([]Caller, 0)
	respone := make([]Caller, 0)
	post := make([]Caller, 0)
	for _, caller := range callers {
		if caller.GetLevel() == LEVEL_PRE {
			pre = append(main, caller)
			continue
		}
		if caller.GetLevel() == LEVEL_POST {
			post = append(main, caller)
			continue
		}
		if caller.GetLevel()&LEVEL_NONE == LEVEL_NONE {
			main = append(main, caller)
		}
		if caller.GetLevel()&LEVEL_CONNECT == LEVEL_CONNECT {
			connect = append(connect, caller)
		}
		if caller.GetLevel()&LEVEL_REQUEST == LEVEL_REQUEST {
			request = append(request, caller)
		}
		if caller.GetLevel()&LEVEL_RESPONSE == LEVEL_RESPONSE {
			respone = append(respone, caller)
		}
	}
	final := make([]Caller, 0)
	final = append(final, connect...)
	final = append(final, pre...)
	final = append(final, request...)
	final = append(final, main...)
	final = append(final, respone...)
	final = append(final, post...)
	return final
}

func Cascade(in *ShadowRequest, callers ...Caller) (*ShadowResponse, Error) {
	if callers == nil {
		return nil, nil
	}
	var (
		out *ShadowResponse
		mut sync.RWMutex
	)
	in.Header.Add("X-Request-Id", uuid.New().String())
	tasks := make(map[string]<-chan *Response)
	ctx := make(map[string]context.Context)
	or, err := in.CloneShadowRequest()
	if err != nil {
		return nil, NewError(err.Error(), http.StatusInternalServerError)
	}
	for _, cal := range callers {
		err := await(cal, &mut, ctx, tasks, in, out)
		if err != nil {
			return nil, err
		}
		if cal.GetIsParallel() {
			spin(cal, &mut, ctx, tasks, in, or)
			continue
		}
		term, res, err := cal.Call(cal.GetContext(), or.RouteValues, in.CloneRequest, or.CloneRequest)
		if err != nil {
			return nil, err
		}
		if term {
			res, err := NewShandowResponse(res)
			if err != nil {
				return nil, NewError(err.Error(), http.StatusInternalServerError)
			}
			return res, nil
		}
		if res == nil {
			continue
		}
		out, err = createOrUpdateResponse(out, res, cal.GetResponseUpdaters())
		if err != nil {
			return nil, err
		}
		tmp, _err := out.CreateRequest()
		if _err != nil {
			return nil, NewError(_err.Error(), http.StatusInternalServerError)
		}

		_err = UpdateRequest(in, tmp.Request, append(cal.GetRequestUpdaters(), ReqUpdateHeader("X-Request-Id")))
		if _err != nil {
			return nil, NewError(_err.Error(), http.StatusInternalServerError)
		}
		in.Reset()
	}
	if out != nil {
		out.Reset()
	}
	return out, nil
}

func await(cal Caller, mut *sync.RWMutex, ctxs map[string]context.Context, tsks map[string]<-chan *Response, in *ShadowRequest, out *ShadowResponse) Error {
	if cal.GetAwaitList() == nil {
		return nil
	}
	for _, task := range cal.GetAwaitList() {
		var err Error
		mut.RLock()
		ch, chFound := tsks[task]
		ctx, ctxFound := ctxs[task]
		mut.RUnlock()
		if !chFound || !ctxFound {
			return NewError("task not found", http.StatusInternalServerError)
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
					Error: NewError(context.DeadlineExceeded.Error(), http.StatusGatewayTimeout),
				}
			}
		}
		if cr.Error != nil {
			return cr.Error
		}
		out, err = createOrUpdateResponse(out, cr.Response, cal.GetResponseUpdaters())
		if err != nil {
			return err
		}
		tmp, _err := out.CreateRequest()
		if _err != nil {
			return NewError(_err.Error(), http.StatusInternalServerError)
		}

		_err = UpdateRequest(in, tmp.Request, cal.GetRequestUpdaters())
		if _err != nil {
			return NewError(_err.Error(), http.StatusInternalServerError)
		}
		in.Reset()
	}
	return nil
}

func spin(cal Caller, mut *sync.RWMutex, ctxs map[string]context.Context, tsks map[string]<-chan *Response, in *ShadowRequest, or *ShadowRequest) {
	ch := make(chan *Response, 1)
	mut.Lock()
	tsks[cal.GetName()] = ch
	ctxs[cal.GetName()] = cal.GetContext()
	mut.Unlock()
	go func() {
		defer close(ch)
		_, r, err := cal.Call(cal.GetContext(), or.RouteValues, in.CloneRequest, or.CloneRequest)
		if err != nil {
			ch <- &Response{
				Error: err,
			}
			return
		}
		ch <- &Response{
			Response: r,
		}
	}()
}

func createOrUpdateResponse(in *ShadowResponse, res *http.Response, ru []ResponseUpdater) (*ShadowResponse, Error) {
	if in == nil {
		res, err := NewShandowResponse(res)
		if err != nil {
			return nil, NewError(err.Error(), http.StatusInternalServerError)
		}
		return res, nil
	}
	err := UpdateResponse(in, res, ru)
	if err != nil {
		return nil, NewError(err.Error(), http.StatusInternalServerError)
	}
	in.Reset()
	return in, nil
}

func (httpError *httpError) Message() string {
	return httpError.message
}

func (httpError *httpError) Status() int {
	return httpError.status
}
