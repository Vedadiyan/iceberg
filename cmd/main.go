package main

import (
	"log"
	"net/http"

	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/listeners"
	"github.com/vedadiyan/iceberg/internal/parsers"
	"github.com/vedadiyan/iceberg/internal/router"
)

func main() {
	ver, conf, err := parsers.Parse()
	if err != nil {
		log.Fatalln(err.Error())
	}
	var server parsers.Server
	switch ver {
	case parsers.VER_V1:
		{
			specV1, ok := conf.(*parsers.SpecV1)
			if !ok {
				log.Fatalln("invalid program")
			}
			server, err = parsers.BuildV1(specV1, handlerFunc)
			if err != nil {
				log.Fatalln(err.Error())
			}
		}
	default:
		{
			log.Fatalln("unsupported api version")
		}
	}
	auto.ForConfigMap().Bootstrap()
	err = server()
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func handlerFunc(conf *filters.Conf) {
	router.DefaultRouteTable().Register(conf.Frontend, "*", func(w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
		if listeners.IsWebSocket(r) {
			listeners.WebSocketHandler(conf, w, r)
			return
		}
		listeners.HttpHandler(conf, w, r, rv)
	})
}
