package proxies

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	Handler interface {
		Handle(w http.ResponseWriter, r *http.Request, rv netio.RouteValues)
	}
	Proxy struct {
		Name    string
		Address *url.URL
		Timeout time.Duration
		Callers []netio.Caller
	}
)

func NewProxy(address *url.URL, callers []netio.Caller) (Handler, error) {
	proxy := new(Proxy)
	proxy.Address = address
	proxy.Callers = callers
	switch proxy.Address.Scheme {
	case "http", "https":
		{
			return NewHttpProxy(proxy), nil
		}
	case "ws", "wss":
		{
			return NewWebSocket(proxy), nil
		}
	}
	return nil, fmt.Errorf("protocol not supported")
}
