package listeners

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/internal/caches"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/errors"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/logger"
	"github.com/vedadiyan/iceberg/internal/router"
)

type (
	Func          func() (bool, error)
	StepFunctions []Func

	Connection struct {
		conf           *common.Conf
		responseWriter http.ResponseWriter
		request        *http.Request
		routeValues    router.RouteValues
		key            string
		requestId      string
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

func (c *Connection) InitializeCache() error {
	if c.conf.Cache == nil {
		return nil
	}
	clone, err := filters.CloneRequest(c.request)
	if err != nil {
		return err
	}
	key, err := c.conf.Cache.Key(c.routeValues, clone)
	if err != nil {
		return err
	}
	c.key = key
	return nil
}

func (c *Connection) Initialize() Func {
	return func() (bool, error) {
		c.requestId = uuid.NewString()
		c.request.Header.Add("X-Request-Id", c.requestId)
		err := c.InitializeCache()
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func (c *Connection) Intercept(level filters.Level) Func {
	return func() (bool, error) {
		err := filters.HandleFilter(c.request, c.conf.Filters, level)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func (c *Connection) GetCache() Func {
	return func() (bool, error) {
		if c.conf.Cache == nil {
			return true, nil
		}
		res, err := c.conf.Cache.Get(c.key)
		if err == nats.ErrKeyNotFound {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if res != nil {
			res, err := caches.Unmarshal(res)
			if err != nil {
				return false, err
			}
			c.request.Header = res.Header
			c.request.Body = io.NopCloser(bytes.NewBuffer(res.Body))
			_, err = c.Finalize()()
			if err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	}
}

func (c *Connection) SetCache() Func {
	return func() (bool, error) {
		if c.conf.Cache == nil {
			return true, nil
		}
		out, err := caches.Marshal(c.request)
		if err != nil {
			return false, err
		}
		c.conf.Cache.Set(c.key, out)
		return true, nil
	}
}

func (c *Connection) Proxy() Func {
	return func() (bool, error) {
		url := *c.request.URL
		r, err := filters.RequestFrom(httpProxy(c.request, c.conf.Backend))
		if err != nil {
			return false, err
		}
		r.URL = &url
		r.Header.Add("X-Request-Id", c.requestId)
		c.request = r
		return true, nil
	}
}

func (c *Connection) Finalize() Func {
	return func() (bool, error) {
		clone, err := filters.CloneRequest(c.request)
		if err != nil {
			return false, err
		}
		for key, values := range clone.Header {
			if strings.ToLower(key) == "content-length" {
				continue
			}
			for _, value := range values {
				c.responseWriter.Header().Add(key, value)
			}
		}
		body, err := io.ReadAll(clone.Body)
		if err != nil {
			return false, err
		}
		_, err = c.responseWriter.Write(body)
		if err != nil {
			return false, err
		}
		return true, nil
	}
}

func HttpHandler(conf *common.Conf, w http.ResponseWriter, r *http.Request, rv router.RouteValues) {
	connection := Connection{
		conf:           conf,
		responseWriter: w,
		request:        r,
		routeValues:    rv,
	}
	err := StepFunctions{
		HandleCORS(conf, w, r),
		connection.Initialize(),
		connection.Intercept(filters.CONNECT),
		connection.GetCache(),
		connection.Intercept(filters.REQUEST),
		connection.Proxy(),
		connection.Intercept(filters.RESPONSE),
		connection.Finalize(),
		connection.SetCache(),
	}.Run()
	if err != nil {
		statusCode := 418
		if handlerError, ok := err.(errors.HandlerError); ok {
			statusCode = handlerError.StatusCode
		}
		http.Error(w, err.Error(), statusCode)
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
			return nil, errors.NewHandlerError(errors.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
		}
		logger.Error(err, "proxy failed", res.StatusCode, string(r))
		return nil, errors.NewHandlerError(errors.HANDLER_ERROR_PROXY, res.StatusCode, res.Status)
	}
	return res, nil
}

func isSuccess(statusCode int) bool {
	return statusCode < 400
}
