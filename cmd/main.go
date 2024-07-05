package main

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/vedadiyan/iceberg/cmd/parser"
	"github.com/vedadiyan/iceberg/cmd/server"
	"github.com/vedadiyan/iceberg/internal/netio"
	"github.com/vedadiyan/iceberg/internal/proxies"
)

func main() {
	config := os.Getenv("ICERBERG_CONFIG")
	_, _, specs, err := parser.Parse([]byte(config))
	if err != nil {
		log.Fatalln(err)
	}
	specsV1 := specs.(*parser.SpecV1)
	err = parser.ParseV1(specsV1.Resources, func(u *url.URL, pattern string, method string, c []netio.Caller) {
		proxy := proxies.NewHttpProxy(u, c)
		server.HandleFunc(pattern, method, func(w http.ResponseWriter, r *http.Request, rv server.RouteValues) {
			proxy.Handle(w, r, func(sr *netio.ShadowRequest) {
				sr.RouteValues = netio.RouteValues(rv)
			})
		})
	})
	if err != nil {
		log.Fatalln(err)
	}
	server.ListenAndServe(specsV1.Listen)
}
