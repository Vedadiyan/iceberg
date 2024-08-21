package logging

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	LoggerFunc func() Logger

	Logger interface {
		Init(*Metadata)
		Trace(*Tracedata)
		Close(bool, error)
	}
	Metadata struct {
		App       string
		Name      string
		Frontend  string
		Backend   string
		User      string
		IpAddress string
	}
	TracedataStatic struct {
		Name   string
		Parent string
		Type   string
		Level  string
	}
	Tracedata struct {
		*TracedataStatic
		Func   string
		Header http.Header
		Url    *url.URL
		RV     netio.RouteValues
		Body   []byte
	}
)

var (
	_loggers sync.Map
)

func GetLogger(name string) Logger {
	if logger, ok := _loggers.Load(name); ok {
		return logger.(LoggerFunc)()
	}
	return nil
}

func AddOrUpdateLogger(n string, l LoggerFunc) {
	_loggers.Store(n, l)
}
