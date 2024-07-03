package main

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/netio"
	"github.com/vedadiyan/iceberg/internal/proxies"
	"github.com/vedadiyan/iceberg/internal/server"
)

func main() {
	target, _ := url.Parse("https://www.google.com")
	proxy := proxies.NewHttpProxy(target, nil)
	callback, _ := url.Parse("nats://127.0.0.1:4222/test")
	f := filters.NewFilter()
	f.Address = callback
	f.Level = netio.LEVEL_REQUEST
	f.SetExchangeHeaders([]string{"X-Test-Header", "New-Header"})
	// f.SetExchangeBody()
	filter, err := filters.NewCoreNATSFilter(filters.NewBaseNATS(f))
	if err != nil {
		panic(err)
	}
	server.HandleFunc("/", func(w http.ResponseWriter, r *http.Request, rv server.RouteValues) {
		shadowRequest, err := netio.NewShadowRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		shadowRequest.RouteValues = netio.RouteValues(rv)
		res, _err := netio.Cascade(shadowRequest, netio.Sort(proxy, filter)...)
		if _err != nil {
			http.Error(w, _err.Message(), _err.Status())
			return
		}
		res.Write(w)
	})

	server.ListenAndServe(":8080")
}
