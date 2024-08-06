package server

import (
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/bootstrap"
	"github.com/vedadiyan/iceberg/internal/common/router"
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

func HandleFunc(pattern string, method string, handler func(w http.ResponseWriter, r *http.Request, rv bootstrap.RouteValues), options ...bootstrap.RegistrationOptions) error {
	url, err := url.Parse(pattern)
	if err != nil {
		return err
	}
	var opt bootstrap.Options
	for _, option := range options {
		option(&opt, router.DefaultRouteTable(), url, handler)
	}
	if len(method) == 0 {
		method = "*"
	}
	handler2 := func(w http.ResponseWriter, r *http.Request, rv bootstrap.RouteValues) {
		if opt.Cors && len(opt.ExposeHeaders) != 0 {
			w.Header().Add("Access-Control-Expose-Headers", opt.ExposeHeaders)
		}
		handler(w, r, rv)
	}
	if method == "*" {
		router.DefaultRouteTable().Register(url, "GET", handler2)
		router.DefaultRouteTable().Register(url, "HEAD", handler2)
		router.DefaultRouteTable().Register(url, "POST", handler2)
		router.DefaultRouteTable().Register(url, "PUT", handler2)
		router.DefaultRouteTable().Register(url, "DELETE", handler2)
		return nil
	}
	router.DefaultRouteTable().Register(url, method, handler2)
	return nil
}

func ListenAndServe(addr string) {
	server := http.Server{
		Addr:              addr,
		ReadTimeout:       time.Second * 10,
		ReadHeaderTimeout: time.Second * 5,
		WriteTimeout:      time.Second * 30,
		Handler:           _mux,
	}
	_ = server.ListenAndServe()
}
