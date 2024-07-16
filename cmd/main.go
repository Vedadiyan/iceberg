package main

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/vedadiyan/iceberg/internal/bootstrap/parser"
	"github.com/vedadiyan/iceberg/internal/bootstrap/server"
	"github.com/vedadiyan/iceberg/internal/callers/proxies"
	"github.com/vedadiyan/iceberg/internal/common/netio"
	"gopkg.in/yaml.v3"
)

func main() {
	test := `agent: 'nats://[default_nats]/$OPA_AGENT'
http:
- test-policy: local
- Ok`
	x := new(parser.OpaV1)
	err := yaml.Unmarshal([]byte(test), x)

	config := os.Getenv("ICERBERG_CONFIG")
	_, _, specs, err := parser.Parse([]byte(config))
	if err != nil {
		log.Fatalln(err)
	}
	specsV1 := specs.(*parser.SpecV1)
	err = parser.ParseV1(specsV1.Resources, func(u *url.URL, pattern string, method string, c []netio.Caller) {
		proxy, err := proxies.NewProxy(u, c)
		if err != nil {
			log.Fatalln(err)
		}
		err = server.HandleFunc(pattern, method, func(w http.ResponseWriter, r *http.Request, rv server.RouteValues) {
			proxy.Handle(w, r, netio.RouteValues(rv))
		})
		if err != nil {
			log.Fatalln(err)
		}
	})
	if err != nil {
		log.Fatalln(err)
	}
	server.ListenAndServe(specsV1.Listen)
}
