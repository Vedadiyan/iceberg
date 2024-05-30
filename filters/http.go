package filters

import (
	"net/http"
)

type (
	HttpFilter struct {
		FilterBase
		Method  string
		Headers map[string]any
	}
)

func (filter *HttpFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r, WithMethod(filter.Method), WithUrl(filter.Address))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
