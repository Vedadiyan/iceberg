package netio

import (
	"fmt"
	"net/http"
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
		Call(Cloner) (*http.Response, error)
		GetRequestUpdaters() []RequestUpdater
		GetResponseUpdaters() []ResponseUpdater
	}
)

func Cascade(r *ShadowRequest, callers ...Caller) (*ShadowResponse, error) {
	var shadowResponse *ShadowResponse
	var err error
	tasks := make(map[string]<-chan *Response)
	for _, caller := range callers {
		if len(caller.GetAwaitList()) != 0 {
			for _, task := range caller.GetAwaitList() {
				ch, ok := tasks[task]
				if !ok {
					return nil, fmt.Errorf("task not found")
				}
				res := <-ch
				if res.error != nil {
					return nil, res.error
				}
				shadowResponse, err = createOrUpdateResponse(shadowResponse, res.Response, caller.GetResponseUpdaters())
				if err != nil {
					return nil, err
				}
				tmp, err := shadowResponse.CreateRequest()
				if err != nil {
					return nil, err
				}

				err = UpdateRequest(r, tmp.Request, caller.GetRequestUpdaters())
				if err != nil {
					return nil, err
				}
				r.Reset()
			}
		}

		if caller.GetIsParallel() {
			go func() {
				ch := make(chan *Response, 1)
				res, err := caller.Call(r.CloneRequest)
				if err != nil {
					ch <- &Response{
						error: err,
					}
					return
				}
				ch <- &Response{
					Response: res,
				}
			}()
			continue
		}
		res, err := caller.Call(r.CloneRequest)
		if err != nil {
			return nil, err
		}
		shadowResponse, err = createOrUpdateResponse(shadowResponse, res, caller.GetResponseUpdaters())
		if err != nil {
			return nil, err
		}
		tmp, err := shadowResponse.CreateRequest()
		if err != nil {
			return nil, err
		}

		err = UpdateRequest(r, tmp.Request, caller.GetRequestUpdaters())
		if err != nil {
			return nil, err
		}
		r.Reset()
	}
	shadowResponse.Reset()
	return shadowResponse, nil
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
