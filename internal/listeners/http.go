package listeners

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/conf"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/logger"
	"github.com/vedadiyan/iceberg/internal/router"
)

type (
	Func          func() (bool, error)
	StepFunctions []Func
)

func (stepFunctions StepFunctions) Run() error {
	for _, fn := range stepFunctions {
		cont, err := fn()
		if err != nil {
			return err
		}
		if !cont {
			break
		}
	}
	return nil
}

func Initializer(conf *conf.Conf, r *http.Request, rv router.RouteValues, requestId *string, key *string) Func {
	return func() (bool, error) {
		*requestId = uuid.NewString()
		r.Header.Add("X-Request-Id", *requestId)
		clone, err := filters.CloneRequest(r)
		if err != nil {
			return false, err
		}
		if conf.Cache != nil {
			_key, err := conf.Cache.Key(rv, clone)
			if err != nil {
				return false, err
			}
			*key = _key
		}
		return true, nil
	}
}

func HandleFilters(conf *conf.Conf, r *http.Request, level filters.Level) Func {
	return func() (bool, error) {
		err := filters.HandleFilter(r, conf.Filters, level)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func GetCache(conf *conf.Conf, w http.ResponseWriter, key string) Func {
	return func() (bool, error) {
		if conf.Cache == nil {
			return true, nil
		}
		res, err := conf.Cache.Get(key)
		if err != nil {
			return false, err
		}
		if res != nil {
			var r http.Request
			err := gob.NewDecoder(bytes.NewBuffer(res)).Decode(&r)
			if err != nil {
				return false, err
			}
			_, err = Finalizer(w, &r)()
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
}

func SetCache(conf *conf.Conf, key string, r *http.Request) Func {
	return func() (bool, error) {
		if conf.Cache == nil {
			return true, nil
		}
		var out bytes.Buffer
		err := gob.NewEncoder(&out).Encode(*r)
		if err != nil {
			return false, err
		}
		conf.Cache.Set(key, out.Bytes())
		return true, nil
	}
}

func HandleProxy(conf *conf.Conf, r *http.Request, requestId string) Func {
	return func() (bool, error) {
		url := *r.URL
		r, err := filters.RequestFrom(httpProxy(r, conf.Backend))
		if err != nil {
			return false, err
		}
		r.URL = &url
		r.Header.Add("X-Request-Id", requestId)
		return true, nil
	}
}

func Finalizer(w http.ResponseWriter, r *http.Request) Func {
	return func() (bool, error) {
		clone, err := filters.CloneRequest(r)
		if err != nil {
			return false, err
		}
		for key, values := range clone.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		body, err := io.ReadAll(clone.Body)
		if err != nil {
			return false, err
		}
		_, err = w.Write(body)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func HttpHandler(conf *conf.Conf, w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
	var key string
	var requestId string
	stepFunctions := StepFunctions{
		HandleCORS(conf, w, r),
		Initializer(conf, r, rv, &requestId, &key),
		HandleFilters(conf, r, filters.CONNECT),
		GetCache(conf, w, key),
		HandleFilters(conf, r, filters.REQUEST),
		HandleProxy(conf, r, requestId),
		HandleFilters(conf, r, filters.RESPONSE),
		Finalizer(w, r),
		SetCache(conf, key, r),
	}
	err := stepFunctions.Run()
	if err != nil {
		if handlerError, ok := err.(common.HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
	}
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
