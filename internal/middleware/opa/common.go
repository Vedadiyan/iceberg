package opa

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	Evaluator interface {
		Eval(*http.Request) (bool, string, error)
	}
	Opa struct {
		AppName  string
		Agent    *url.URL
		Timeout  time.Duration
		Policies map[string]PolicyType
	}
	Union[T any] struct {
		Error error
		Value T
	}
)

func (opa *Opa) GetLevel() netio.Level {
	return netio.LEVEL_CONNECT
}

func (opa *Opa) GetIsParallel() bool {
	return false
}

func (opa *Opa) GetName() string {
	return ""
}

func (opa *Opa) GetAwaitList() []string {
	return nil
}

func (opa *Opa) GetRequestUpdaters() []netio.RequestUpdater {
	return nil
}

func (opa *Opa) GetResponseUpdaters() []netio.ResponseUpdater {
	return nil
}

func (opa *Opa) GetContext() context.Context {
	return context.TODO()
}
