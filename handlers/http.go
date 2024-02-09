package handlers

import (
	"net/http"
)

type (
	HttpFilterBase struct {
		FilterBase
		Method string
	}
	HttpRequestFilter struct {
		HttpFilterBase
	}
	HttpResponseFilter struct {
		HttpFilterBase
	}
)

func (filter *HttpFilterBase) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r, WithMethod(filter.Method), WithUrl(filter.Address))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func (filter *HttpRequestFilter) Level() Level {
	return INTERCEPT
}

func (filter *HttpResponseFilter) Level() Level {
	return POST_PROCESS
}
