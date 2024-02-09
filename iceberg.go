package main

import (
	"io"
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/handlers"
)

type (
	StatusCodeClass int
	Handler         func(*http.ServeMux)
)

const (
	NONE         StatusCodeClass = 0
	SUCCESS      StatusCodeClass = 1
	CLIENT_ERROR StatusCodeClass = 2
	SERVER_ERROR StatusCodeClass = 3
)

func New(conf handlers.Conf) Handler {
	return func(sm *http.ServeMux) {
		sm.HandleFunc(conf.Frontend.String(), func(w http.ResponseWriter, r *http.Request) {
			err := FilterRequest(r, conf.Filters)
			if err != nil {
				return
			}
			res, err := HttpProxy(r, conf.Backend)
			if err != nil {
				return
			}
			req, err := handlers.ConvertToRequest(res, r)
			if err != nil {
				return
			}
			err = FilterResponse(req, conf.Filters)
			if err != nil {
				return
			}
			for key, values := range req.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return
			}
			w.Write(body)
		})
	}
}

func HttpProxy(r *http.Request, backend url.URL) (*http.Response, error) {
	req, err := handlers.CloneRequest(r, handlers.WithUrl(backend))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)

}

func HandlerFunc(r *http.Request, filter handlers.Filter) error {
	res, err := filter.Handle(r)
	if err != nil {
		return err
	}
	if StatusCodeClass(res.StatusCode) != SUCCESS {
		return err

	}
	err = filter.MoveTo(res, r)
	if err != nil {
		return err
	}
	return nil
}

func FilterRequest(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.INTERCEPT {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}
func FilterResponse(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.POST_PROCESS {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func Success(statusCode int) StatusCodeClass {
	if statusCode < 200 {
		return NONE
	}
	if statusCode < 400 {
		return SUCCESS
	}
	if statusCode < 500 {
		return CLIENT_ERROR
	}
	return SERVER_ERROR
}
