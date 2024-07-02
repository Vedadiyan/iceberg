package main

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/netio"
	"github.com/vedadiyan/iceberg/internal/proxies"
)

func main() {
	filterUrl, err := url.Parse("http://127.0.0.1:8081")
	if err != nil {
		panic(err)
	}
	targetUrl, err := url.Parse("http://www.google.com")
	if err != nil {
		panic(err)
	}
	filter := filters.NewHttpFilter(&filters.Filter{
		Address: filterUrl,
		Level:   2,
	})
	proxy := proxies.NewHttpProxy(targetUrl, []netio.Caller{filter})
	_ = proxy
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := proxy.Handle(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		res.Write(w)
	})
	http.ListenAndServe(":8080", nil)
}

func CreateRequest(r *http.Request) (*netio.ShadowRequest, error) {
	shadowReequest, err := netio.NewShadowRequest(r)
	if err != nil {
		return nil, err
	}
	return shadowReequest, nil
}
