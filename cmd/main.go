package main

import (
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/cache"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/netio"
	"github.com/vedadiyan/iceberg/internal/proxies"
	"github.com/vedadiyan/iceberg/internal/server"
)

func main() {
	jetstream, _ := url.Parse("nats://127.0.0.1:4222/test")
	cache, _ := cache.NewJetStream(&cache.Cache{
		Address:     jetstream,
		TTL:         time.Second * 30,
		KeyTemplate: "test",
	})
	_ = cache
	fcallback, _ := url.Parse("nats://127.0.0.1:4222/test2")

	ff := filters.NewFilter()
	ff.Name = "test"
	ff.Address = fcallback
	ff.Parallel = true
	ff.Level = netio.LEVEL_NONE
	ffilter, err := filters.NewCoreNATSFilter(filters.NewBaseNATS(ff))
	if err != nil {
		panic(err)
	}

	ffcallback, _ := url.Parse("nats://127.0.0.1:4222/test3")

	fff := filters.NewFilter()
	fff.Address = ffcallback
	fff.Level = netio.LEVEL_NONE
	fff.AwaitList = []string{"test"}
	fffilter, err := filters.NewCoreNATSFilter(filters.NewBaseNATS(fff))
	if err != nil {
		panic(err)
	}

	target, _ := url.Parse("http://127.0.0.1:8081")
	proxy := proxies.NewHttpProxy(target, nil)
	callback, _ := url.Parse("nats://127.0.0.1:4222/abc")
	f := filters.NewFilter()
	f.Address = callback
	f.Level = netio.LEVEL_REQUEST
	f.Callers = append(f.Callers, ffilter)
	f.Callers = append(f.Callers, fffilter)
	f.SetExchangeHeaders([]string{"X-Test-Header", "New-Header"})
	// f.SetExchangeBody()
	filter, err := filters.NewDurableNATSFilter(filters.NewBaseNATS(f))
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
