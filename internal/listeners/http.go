package listeners

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
)

func HttpHandler(conf *filters.Conf, w http.ResponseWriter, r *http.Request) {
	if HandleCORS(conf, w, r) {
		return
	}

	log.Println("handling request", r.URL.String(), r.Method)
	log.Println("handling request filters")
	err := filters.HandleFilter(r, conf.Filters, filters.REQUEST)
	if err != nil {
		log.Println("request filter failed", err)
		if handlerError, ok := err.(common.HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
		return
	}
	log.Println("handling proxy")
	url := *r.URL
	r, err = filters.RequestFrom(httpProxy(r, conf.Backend))
	if err != nil {
		log.Println(err)
		w.WriteHeader(502)
		return
	}
	r.URL = &url
	log.Println("handling response filters")
	err = filters.HandleFilter(r, conf.Filters, filters.RESPONSE)
	if err != nil {
		log.Println("response filter failed", err)
		if handlerError, ok := err.(common.HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
		return
	}
	for key, values := range r.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(body)
}

func httpProxy(r *http.Request, backend *url.URL) (*http.Response, error) {
	req, err := filters.CloneRequest(r, filters.WithUrl(backend), filters.WithMethod(r.Method))
	if err != nil {
		log.Println("proxy failed", err)
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("proxy failed", err)
		return nil, err
	}
	if res.StatusCode%200 >= 100 && strings.ToLower(res.Header.Get(string(filters.HEADER_CONTINUE_ON_ERROR))) != "true" {
		r, err := io.ReadAll(res.Body)
		if err != nil {
			log.Println("proxy failed", res.StatusCode, "unknown")
			return nil, common.NewHandlerError(common.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
		}
		log.Println("proxy failed", res.StatusCode, string(r))
		return nil, common.NewHandlerError(common.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
	}
	return res, nil
}
