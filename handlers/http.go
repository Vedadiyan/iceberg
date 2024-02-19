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

func (filter *HttpFilter) Is(level Level) bool {
	return filter.Level&level == level
}

func (filter *HttpFilter) MoveTo(res *http.Response, req *http.Request) error {
	if filter.Is(PARALLEL) {
		return nil
	}
	for _, header := range filter.ExchangeHeaders {
		values := res.Header[header]
		if len(values) > 0 {
			for _, value := range values {
				req.Header.Add(header, value)
			}
		}
	}
	if filter.ExchangeBody {
		req.Body = res.Body
	}
	return nil
}
