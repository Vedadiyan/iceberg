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
	OpaType string
	Opa     struct {
		AppName  string
		Agent    *url.URL
		Timeout  time.Duration
		Policies map[string]PolicyType
		Type     OpaType
	}
	Union[T any] struct {
		Error error
		Value T
	}
)

const (
	OPA_TYPE_HTTP       OpaType = "http"
	OPA_TYPE_WS_SEND    OpaType = "send"
	OPA_TYPE_WS_RECEIVE OpaType = "receive"
)

func (opa *Opa) GetLevel() netio.Level {
	switch opa.Type {
	case OPA_TYPE_HTTP:
		{
			return netio.LEVEL_CONNECT
		}
	case OPA_TYPE_WS_SEND:
		{
			return netio.LEVEL_REQUEST
		}
	case OPA_TYPE_WS_RECEIVE:
		{
			return netio.LEVEL_RESPONSE
		}
	}
	return netio.LEVEL_NONE
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
