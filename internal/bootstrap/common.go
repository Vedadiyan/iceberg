package bootstrap

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/common/router"
)

type (
	RouteValues         = router.RouteValues
	RegistrationOptions func(*Options, *router.RouteTable, *url.URL, func(w http.ResponseWriter, r *http.Request, rv RouteValues))
	Options             struct {
		Cors          bool
		ExposeHeaders string
	}
	CORS struct {
		AllowedOrigins string
		AllowedHeaders string
		AllowedMethods string
		MaxAge         string
		ExposedHeaders string
	}
)

func WithCORSDisabled() RegistrationOptions {
	return func(opt *Options, rt *router.RouteTable, u *url.URL, f func(w http.ResponseWriter, r *http.Request, rv RouteValues)) {
		opt.Cors = true
		opt.ExposeHeaders = "*"
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
		opt.Cors = true
		opt.ExposeHeaders = cors.ExposedHeaders
		rt.Register(u, "OPTIONS", func(w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
			w.Header().Add("access-control-allow-origin", cors.AllowedOrigins)
			w.Header().Add("access-control-allow-headers", cors.AllowedHeaders)
			w.Header().Add("access-control-max-age", cors.MaxAge)
			w.Header().Add("access-control-allow-methods", cors.AllowedMethods)
			w.WriteHeader(200)
		})
	}
}
