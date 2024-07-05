package server

import (
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/common/router"
)

type (
	RouteValues         = router.RouteValues
	RegistrationOptions func(*Options, *router.RouteTable, *url.URL, func(w http.ResponseWriter, r *http.Request, rv RouteValues))
	Options             struct {
		cors          bool
		exposeHeaders string
	}
	CORS struct {
		AllowedOrigins string
		AllowedHeaders string
		AllowedMethods string
		MaxAge         string
		ExposedHeaders string
	}
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

func WithCORSDisabled() RegistrationOptions {
	return func(opt *Options, rt *router.RouteTable, u *url.URL, f func(w http.ResponseWriter, r *http.Request, rv RouteValues)) {
		opt.cors = true
		opt.exposeHeaders = "*"
		rt.Register(u, "OPTIONS", func(w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
			w.Header().Add("access-control-allow-origin", "*")
			w.Header().Add("access-control-allow-headers", "*")
			w.Header().Add("access-control-max-age", "3628800")
			w.Header().Add("access-control-allow-methods", "GET, DELETE, OPTIONS, POST, PUT")
			w.WriteHeader(200)
		})
	}
}

func WithCORS(cors *CORS) RegistrationOptions {
	return func(opt *Options, rt *router.RouteTable, u *url.URL, f func(w http.ResponseWriter, r *http.Request, rv RouteValues)) {
		opt.cors = true
		opt.exposeHeaders = cors.ExposedHeaders
		rt.Register(u, "OPTIONS", func(w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
			w.Header().Add("access-control-allow-origin", cors.AllowedOrigins)
			w.Header().Add("access-control-allow-headers", cors.AllowedHeaders)
			w.Header().Add("access-control-max-age", cors.MaxAge)
			w.Header().Add("access-control-allow-methods", cors.AllowedMethods)
			w.WriteHeader(200)
		})
	}
}

func HandleFunc(pattern string, method string, handler func(w http.ResponseWriter, r *http.Request, rv RouteValues), options ...RegistrationOptions) error {
	url, err := url.Parse(pattern)
	if err != nil {
		return err
	}
	var opt Options
	for _, option := range options {
		option(&opt, router.DefaultRouteTable(), url, handler)
	}
	if len(method) == 0 {
		method = "*"
	}
	handler = func(w http.ResponseWriter, r *http.Request, rv RouteValues) {
		if opt.cors && len(opt.exposeHeaders) != 0 {
			w.Header().Add("Access-Control-Expose-Headers", opt.exposeHeaders)
		}
		handler(w, r, rv)
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
	server := http.Server{
		Addr:              addr,
		ReadTimeout:       time.Second * 10,
		ReadHeaderTimeout: time.Second * 5,
		WriteTimeout:      time.Second * 30,
		Handler:           _mux,
	}
	_ = server.ListenAndServe()
}
