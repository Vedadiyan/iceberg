package netio

import (
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
		Call(Cloner) (*http.Response, error)
		GetRequestUpdaters() []RequestUpdater
		GetResponseUpdaters() []ResponseUpdater
	}
)

func Cascade(i *ShadowRequest, callers ...Caller) (*ShadowResponse, error) {
	var (
		o   *ShadowResponse
		mut sync.RWMutex
	)
	tasks := make(map[string]<-chan *Response)

	for _, c := range callers {
		err := await(c, &mut, tasks, i, o)
		if err != nil {
			return nil, err
		}
		if c.GetIsParallel() {
			spin(c, &mut, tasks, i)
			continue
		}
		r, err := c.Call(i.CloneRequest)
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

func await(c Caller, m *sync.RWMutex, t map[string]<-chan *Response, i *ShadowRequest, o *ShadowResponse) error {
	if len(c.GetAwaitList()) != 0 {
		for _, task := range c.GetAwaitList() {
			var err error
			m.RLocker()
			ch, ok := t[task]
			m.RUnlock()
			if !ok {
				return fmt.Errorf("task not found")
			}
			cr := <-ch
			if cr.error != nil {
				return cr.error
			}
			o, err = createOrUpdateResponse(o, cr.Response, c.GetResponseUpdaters())
			if err != nil {
				return err
			}
			tmp, err := o.CreateRequest()
			if err != nil {
				return err
			}

			err = UpdateRequest(i, tmp.Request, c.GetRequestUpdaters())
			if err != nil {
				return err
			}
			i.Reset()
		}
	}
	return nil
}

func spin(c Caller, m *sync.RWMutex, t map[string]<-chan *Response, i *ShadowRequest) {
	go func() {
		ch := make(chan *Response, 1)
		m.Lock()
		t[c.GetName()] = ch
		m.Unlock()
		r, err := c.Call(i.CloneRequest)
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
