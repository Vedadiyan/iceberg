package listeners

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/logger"
)

func HttpHandler(conf *filters.Conf, w http.ResponseWriter, r *http.Request) {
	if HandleCORS(conf, w, r) {
		return
	}
	requestId := uuid.NewString()
	r.Header.Add("X-Request-Id", requestId)
	logger.Info("handling request", r.URL.String(), r.Method)

	if conf.Auth != nil {
		logger.Info("handling auth filter")
		err := filters.HandleFilter(r, []filters.Filter{conf.Auth}, filters.REQUEST)
		if err != nil {
			logger.Error(err, "auth filter failed")
			if handlerError, ok := err.(common.HandlerError); ok {
				w.WriteHeader(handlerError.StatusCode)
				w.Write([]byte(handlerError.Message))
				return
			}
			w.WriteHeader(418)
			return
		}
	}

	logger.Info("handling request filters")
	err := filters.HandleFilter(r, conf.Filters, filters.REQUEST)
	if err != nil {
		logger.Error(err, "request filter failed")
		if handlerError, ok := err.(common.HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
		return
	}
	logger.Info("handling proxy")
	url := *r.URL
	r, err = filters.RequestFrom(httpProxy(r, conf.Backend))
	if err != nil {
		logger.Error(err, "")
		w.WriteHeader(502)
		return
	}
	r.URL = &url
	r.Header.Add("X-Request-Id", requestId)
	log.Println("handling response filters")
	err = filters.HandleFilter(r, conf.Filters, filters.RESPONSE)
	if err != nil {
		logger.Error(err, "response filter failed")
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
		logger.Error(err, "proxy failed")
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error(err, "proxy failed")
		return nil, err
	}
	if !isSuccess(res.StatusCode) && strings.ToLower(res.Header.Get(string(filters.HEADER_CONTINUE_ON_ERROR))) != "true" {
		r, err := io.ReadAll(res.Body)
		if err != nil {
			logger.Error(err, "proxy failed", res.StatusCode, "unknown")
			return nil, common.NewHandlerError(common.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
		}
		logger.Error(err, "proxy failed", res.StatusCode, string(r))
		return nil, common.NewHandlerError(common.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
	}
	return res, nil
}

func isSuccess(statusCode int) bool {
	return statusCode < 400
}
