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

func (filter *HttpFilter) HandleSync(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r, WithMethod(filter.Method), WithUrl(filter.Address))
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if len(filter.Filters) > 0 {
		req, err := RequestFrom(res, nil)
		if err != nil {
			return nil, err
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (filter *HttpFilter) HandleAsync(r *http.Request) {
	go filter.HandleSync(r)
}
