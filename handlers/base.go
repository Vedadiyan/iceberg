package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

type (
	Level  int
	Filter interface {
		Handle(r *http.Request) (*http.Response, error)
		Is(level Level) bool
		MoveTo(*http.Response, *http.Request) error
	}
	FilterBase struct {
		Filter
		Address         *url.URL
		ExchangeHeaders []string
		DropHeaders     []string
		ExchangeBody    bool
		Level           Level
		Timeout         int
	}
	Conf struct {
		Frontend *url.URL
		Backend  *url.URL
		Filters  []Filter
	}
	Request struct {
		Url    *url.URL
		Method string
	}
	RequestOption func(*Request)
)

const (
	REQUEST  Level = 2
	RESPONSE Level = 4
	CONNECT  Level = 8
	PARALLEL Level = 16
)

func CloneRequest(r *http.Request, options ...RequestOption) (*http.Request, error) {
	clone := r.Clone(context.TODO())
	body, err := io.ReadAll(clone.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	clone.Body = io.NopCloser(bytes.NewBuffer(body))
	return clone, nil
}

func RequestFrom(res *http.Response, err error) (*http.Request, error) {
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequest("", "", res.Body)
	if err != nil {
		return nil, err
	}
	for key, values := range res.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	return r, nil
}

func WithUrl(url *url.URL) RequestOption {
	return func(r *Request) {
		r.Url = url
	}
}

func WithMethod(method string) RequestOption {
	return func(r *Request) {
		r.Method = method
	}
}

func (filter *FilterBase) Is(level Level) bool {
	return filter.Level&level == level
}

func (filter *FilterBase) MoveTo(res *http.Response, req *http.Request) error {
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
