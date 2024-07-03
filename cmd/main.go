package main

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/netio"
	"github.com/vedadiyan/iceberg/internal/proxies"
	"github.com/vedadiyan/iceberg/internal/server"
)

func main() {
	url, _ := url.Parse("https://www.google.com")
	proxy := proxies.NewHttpProxy(url, nil)
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request, rv server.RouteValues) {
		shadowRequest, err := netio.NewShadowRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		shadowRequest.RouteValues = netio.RouteValues(rv)
		res, _err := netio.Cascade(shadowRequest, proxy)
		if _err != nil {
			http.Error(w, _err.Message(), _err.Status())
			return
		}
		res.Write(w)
	})

	server.ListenAndServe(":8080")
}
