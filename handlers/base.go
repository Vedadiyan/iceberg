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
	INTERCEPT           Level = "intercept"
	INTERCEPT_SOCKET    Level = "intercept_socket"
	POST_PROCESS        Level = "post_process"
	POST_PROCESS_SOCKET Level = "post_process_socket"
	PARALLEL            Level = "parallel"
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
