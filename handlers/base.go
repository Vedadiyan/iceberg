package handlers

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
)

type (
	Level  string
	Filter interface {
		Handle(r *http.Request) (*http.Response, error)
		Level() Level
		MoveTo(*http.Response, *http.Request) error
	}
	FilterBase struct {
		Filter
		Address         url.URL
		ExchangeHeaders []string
		DropHeaders     []string
		ExchangeBody    bool
	}
	Conf struct {
		Frontend url.URL
		Backend  url.URL
		Filters  []Filter
	}
	Request struct {
		Url    url.URL
		Method string
	}
	RequestOption func(*Request)
)

const (
	INTERCEPT    Level = "intercept"
	POST_PROCESS Level = "post_process"
	PARALLEL     Level = "parallel"
)

func CloneRequest(r *http.Request, options ...RequestOption) (*http.Request, error) {
	request := Request{
		Url:    *r.URL,
		Method: r.Method,
	}
	for _, option := range options {
		option(&request)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	(*r).Body = io.NopCloser(bytes.NewBuffer(body))
	return http.NewRequest(request.Method, request.Url.String(), io.NopCloser(bytes.NewBuffer(body)))
}

func ConvertToRequest(res *http.Response, req *http.Request) (*http.Request, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	(*req).Body = io.NopCloser(bytes.NewBuffer(body))
	clone, err := http.NewRequest("", "", io.NopCloser(bytes.NewBuffer(body)))
	if err != nil {
		return nil, err
	}
	for key, values := range req.Header {
		for _, value := range values {
			clone.Header.Add(key, value)
		}
	}
	return clone, nil
}

func WithUrl(url url.URL) RequestOption {
	return func(r *Request) {
		r.Url = url
	}
}

func WithMethod(method string) RequestOption {
	return func(r *Request) {
		r.Method = method
	}
}
