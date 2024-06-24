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

	Connection struct {
		conf *conf.Conf
		w    http.ResponseWriter
		r    *http.Request
		rv   router.RouteValues
	}
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

func (c *Connection) Initialize(requestId *string, key *string) Func {
	return func() (bool, error) {
		*requestId = uuid.NewString()
		c.r.Header.Add("X-Request-Id", *requestId)
		if c.conf.Cache != nil {
			clone, err := filters.CloneRequest(c.r)
			if err != nil {
				return false, err
			}
			_key, err := c.conf.Cache.Key(c.rv, clone)
			if err != nil {
				return false, err
			}
			*key = _key
		}
		return true, nil
	}
}

func (c *Connection) Intercept(level filters.Level) Func {
	return func() (bool, error) {
		err := filters.HandleFilter(c.r, c.conf.Filters, level)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func (c *Connection) GetCache(key string) Func {
	return func() (bool, error) {
		if c.conf.Cache == nil {
			return true, nil
		}
		res, err := c.conf.Cache.Get(key)
		if err != nil {
			return false, err
		}
		if res != nil {
			var r http.Request
			err := gob.NewDecoder(bytes.NewBuffer(res)).Decode(&r)
			if err != nil {
				return false, err
			}
			_, err = c.Finalize()()
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
}

func (c *Connection) SetCache(key string) Func {
	return func() (bool, error) {
		if c.conf.Cache == nil {
			return true, nil
		}
		var out bytes.Buffer
		err := gob.NewEncoder(&out).Encode(*c.r)
		if err != nil {
			return false, err
		}
		c.conf.Cache.Set(key, out.Bytes())
		return true, nil
	}
}

func (c *Connection) Proxy(requestId string) Func {
	return func() (bool, error) {
		url := *c.r.URL
		r, err := filters.RequestFrom(httpProxy(c.r, c.conf.Backend))
		if err != nil {
			return false, err
		}
		r.URL = &url
		r.Header.Add("X-Request-Id", requestId)
		return true, nil
	}
}

func (c *Connection) Finalize() Func {
	return func() (bool, error) {
		clone, err := filters.CloneRequest(c.r)
		if err != nil {
			return false, err
		}
		for key, values := range clone.Header {
			for _, value := range values {
				c.w.Header().Add(key, value)
			}
		}
		body, err := io.ReadAll(clone.Body)
		if err != nil {
			return false, err
		}
		_, err = c.w.Write(body)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func HttpHandler(conf *conf.Conf, w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
	var key string
	var requestId string
	connection := Connection{
		conf: conf,
		w:    w,
		r:    r,
		rv:   rv,
	}
	err := StepFunctions{
		HandleCORS(conf, w, r),
		connection.Initialize(&requestId, &key),
		connection.Intercept(filters.CONNECT),
		connection.GetCache(key),
		connection.Intercept(filters.REQUEST),
		connection.Proxy(requestId),
		connection.Intercept(filters.RESPONSE),
		connection.Finalize(),
		connection.SetCache(key),
	}.Run()
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
