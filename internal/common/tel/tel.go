package tel

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	LoggerFunc func() Telemetry

	Telemetry interface {
		Init(*Metadata)
		Trace(string, any)
		Close(error)
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

	TelemetryOption func(Telemetry)
)

var (
	_loggers sync.Map
)

func TraceRef[T any](name string, value *T) TelemetryOption {
	return func(t Telemetry) {}
}

func Trace[T any](name string, value T) TelemetryOption {
	return func(t Telemetry) {}
}

func Measure[T int | int16 | int32 | int64 | int8 | byte | uint | uint16 | uint32 | uint64 | float32 | float64](name string, value *T) TelemetryOption {
	return func(t Telemetry) {}
}

func Metric[T int | int16 | int32 | int64 | int8 | byte | uint | uint16 | uint32 | uint64 | float32 | float64](name string, value T) TelemetryOption {
	return func(t Telemetry) {}
}

func Open(name string, metadata *Metadata, opts ...TelemetryOption) Telemetry {
	if logger, ok := _loggers.Load(name); ok {
		return logger.(LoggerFunc)()
	}
	return nil
}

func AddOrUpdateLogger(n string, l LoggerFunc) {
	_loggers.Store(n, l)
}

func Request(r *http.Request) (string, *http.Request) {
	return "req", r
}

func Response(r *http.Response) (string, *http.Response) {
	return "res", r
}

func Self[T any](v *T) (string, *T) {
	return "self", v
}

func Func(f string) (string, string) {
	return "func", f
}

func Path(rv netio.RouteValues) (string, netio.RouteValues) {
	return "func", rv
}

func Next(n *netio.Next) (string, *netio.Next) {
	return "next", n
}
