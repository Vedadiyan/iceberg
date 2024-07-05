package server

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/common/router"
)

type (
	RouteValues         = router.RouteValues
	RegistrationOptions func(*router.RouteTable, *url.URL, func(w http.ResponseWriter, r *http.Request, rv RouteValues))
)

var (
	_mux *http.ServeMux
)

func init() {
	_mux = http.NewServeMux()
	_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		router, err := router.DefaultRouteTable().Find(r.URL, r.Method)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		router.ServeHTTP(w, r)
	})
}

func WithCors() RegistrationOptions {
	return func(rt *router.RouteTable, u *url.URL, f func(w http.ResponseWriter, r *http.Request, rv RouteValues)) {
		rt.Register(u, "OPTION", f)
	}
}

func HandleFunc(pattern string, method string, handler func(w http.ResponseWriter, r *http.Request, rv RouteValues), options ...RegistrationOptions) error {
	url, err := url.Parse(pattern)
	if err != nil {
		return err
	}
	for _, option := range options {
		option(router.DefaultRouteTable(), url, handler)
	}
	if len(method) == 0 {
		method = "*"
	}
	if method == "*" {
		router.DefaultRouteTable().Register(url, "GET", handler)
		router.DefaultRouteTable().Register(url, "HEAD", handler)
		router.DefaultRouteTable().Register(url, "POST", handler)
		router.DefaultRouteTable().Register(url, "PUT", handler)
		router.DefaultRouteTable().Register(url, "DELETE", handler)
		return nil
	}
	router.DefaultRouteTable().Register(url, method, handler)
	return nil
}

func ListenAndServe(addr string) {
	_ = http.ListenAndServe(addr, _mux)
}
