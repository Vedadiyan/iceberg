package opa

import (
	"net/http"
	"net/url"
	"time"
)

type (
	Evaluator interface {
		Eval(*http.Request) (bool, string, error)
	}
	Opa struct {
		AppName string
		Agent   *url.URL
		Timeout time.Duration
	}
	Union[T any] struct {
		Error error
		Value T
	}
)
