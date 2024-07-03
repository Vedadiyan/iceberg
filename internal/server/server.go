package server

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/router"
)

type (
	RouteValues = router.RouteValues
)

var (
	_mux *http.ServeMux
)

func init() {
	_mux = http.NewServeMux()
	_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		router, err := router.DefaultRouteTable().Find(r.URL, "*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		router.ServeHTTP(w, r)
	})
}

func HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request, rv RouteValues)) error {
	url, err := url.Parse(pattern)
	if err != nil {
		return err
	}
	router.DefaultRouteTable().Register(url.JoinPath(), "*", handler)
	return nil
}

func ListenAndServe(addr string) {
	_ = http.ListenAndServe(addr, _mux)
}
