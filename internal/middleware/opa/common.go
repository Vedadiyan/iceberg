package opa

import (
	"net/http"
	"net/url"
)

type (
	Evaluator interface {
		Eval(*http.Request) (bool, string, error)
	}
	Opa struct {
		AppName string
		Agent   *url.URL
	}
	Union[T any] struct {
		Error error
		Value T
	}
)
