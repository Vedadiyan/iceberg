package handlers

import (
	"net/http"
)

type (
	HttpFilter struct {
		FilterBase
		Method string
	}
)

func (filter *HttpFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r, WithMethod(filter.Method), WithUrl(filter.Address))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
