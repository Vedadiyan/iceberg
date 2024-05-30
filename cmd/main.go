package main

import (
	"log"
	"net/http"

	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/listeners"
	"github.com/vedadiyan/iceberg/internal/parsers"
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

func handlerFunc(conf *filters.Conf) common.Handler {
	return func(sm *http.ServeMux) {
		sm.HandleFunc(conf.Frontend.String(), func(w http.ResponseWriter, r *http.Request) {
			if listeners.IsWebSocket(r) {
				listeners.WebSocketHandler(conf, w, r)
				return
			}
			listeners.HttpHandler(conf, w, r)
		})
	}
}
