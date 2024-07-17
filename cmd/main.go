package main

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/vedadiyan/iceberg/internal/bootstrap"
	"github.com/vedadiyan/iceberg/internal/bootstrap/parser"
	"github.com/vedadiyan/iceberg/internal/bootstrap/server"
	"github.com/vedadiyan/iceberg/internal/callers/proxies"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

const test = `
package example

import rego.v1

# METADATA
# title: test
allow if {
	true
}
`

func main() {
	os.Setenv("test-policy", test)

	data, err := os.ReadFile("../examples/test.yml")
	if err != nil {
		log.Fatalln(err)
	}
	os.Setenv("ICERBERG_CONFIG", string(data))

	config := os.Getenv("ICERBERG_CONFIG")
	_, _, specs, err := parser.Parse([]byte(config))
	if err != nil {
		log.Fatalln(err)
	}
	specsV1 := specs.(*parser.SpecV1)
	err = parser.ParseV1(specsV1.Resources, func(u *url.URL, pattern string, method string, c []netio.Caller, opts ...bootstrap.RegistrationOptions) {
		proxy, err := proxies.NewProxy(u, c)
		if err != nil {
			log.Fatalln(err)
		}
		err = server.HandleFunc(pattern, method, func(w http.ResponseWriter, r *http.Request, rv bootstrap.RouteValues) {
			proxy.Handle(w, r, netio.RouteValues(rv))
		}, opts...)
		if err != nil {
			log.Fatalln(err)
		}
	})
	if err != nil {
		log.Fatalln(err)
	}
	server.ListenAndServe(specsV1.Listen)
}
