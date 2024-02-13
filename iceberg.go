package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/handlers"
)

type (
	StatusCodeClass   int
	Handler           func(*http.ServeMux)
	HandlerErrorClass int
	HandlerError      struct {
		Class      HandlerErrorClass
		StatusCode int
		Message    string
	}
)

const (
	STATUS_NONE         StatusCodeClass = 0
	STATUS_SUCCESS      StatusCodeClass = 1
	STATUS_CLIENT_ERROR StatusCodeClass = 2
	STATUS_SERVER_ERROR StatusCodeClass = 3

	HANDLER_ERROR_INTERNAL HandlerErrorClass = 0
	HANDLER_ERROR_FILTER   HandlerErrorClass = 1
	HANDLER_ERROR_PROXY    HandlerErrorClass = 2
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func (handlerError HandlerError) Error() string {
	return fmt.Sprintf("%d: %s", handlerError.Class, handlerError.Message)
}

func NewHandlerError(class HandlerErrorClass, statusCode int, message string) error {
	handlerError := HandlerError{
		Class:   class,
		Message: message,
	}
	return handlerError
}

func New(conf handlers.Conf) Handler {
	return func(sm *http.ServeMux) {
		sm.HandleFunc(conf.Frontend.String(), func(w http.ResponseWriter, r *http.Request) {
			if IsWebSocket(r) {
				WebSocketHandler(conf, w, r)
				return
			}
			HttpHandler(conf, w, r)
		})
	}
}

func IsWebSocket(r *http.Request) bool {
	for _, value := range r.Header["Upgrade"] {
		if value == "websocket" {
			return true
		}
	}
	return false
}

func HttpHandler(conf handlers.Conf, w http.ResponseWriter, r *http.Request) {
	err := FilterRequest(r, conf.Filters)
	if err != nil {
		if handlerError, ok := err.(HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
		return
	}
	r, err = handlers.RequestFrom(HttpProxy(r, conf.Backend))
	if err != nil {
		return
	}
	err = FilterResponse(r, conf.Filters)
	if err != nil {
		return
	}
	for key, values := range r.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	w.Write(body)
}

func WebSocketHandler(conf handlers.Conf, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	proxiedConn, _, err := websocket.DefaultDialer.Dial(conf.Backend.String(), nil)
	if err != nil {
		return
	}
	interceptListen := true
	proxyListen := true
	go func() {
		defer func() {
			conn.Close()
			proxyListen = false
		}()
		for interceptListen {
			messageType, data, err := conn.NextReader()
			if err != nil {
				return
			}
			message, err := io.ReadAll(data)
			if err != nil {
				conn.WriteJSON(HandlerError{
					Class:      HANDLER_ERROR_INTERNAL,
					StatusCode: 500,
					Message:    err.Error(),
				})
				continue
			}
			req, err := handlers.CloneRequest(r)
			if err != nil {
				conn.WriteJSON(HandlerError{
					Class:      HANDLER_ERROR_INTERNAL,
					StatusCode: 500,
					Message:    err.Error(),
				})
				return
			}
			req.Body = io.NopCloser(bytes.NewBuffer(message))
			err = FilterRequest(req, conf.Filters)
			if err != nil {
				if handlerError, ok := err.(HandlerError); ok {
					conn.WriteJSON(handlerError)
					return
				}
				conn.WriteJSON(HandlerError{
					StatusCode: 418,
				})
				return
			}
			proxiedConn.SetWriteDeadline(time.Now().Add(time.Second * 2))
			err = proxiedConn.WriteMessage(messageType, message)
			if err != nil {
				conn.WriteJSON(HandlerError{
					Class:      HANDLER_ERROR_PROXY,
					StatusCode: 500,
					Message:    err.Error(),
				})
				continue
			}
		}
	}()
	go func() {
		defer func() {
			proxiedConn.Close()
			interceptListen = false
		}()
		for proxyListen {
			messageType, data, err := proxiedConn.NextReader()
			if err != nil {
				return
			}
			message, err := io.ReadAll(data)
			if err != nil {
				continue
			}
			req, err := http.NewRequest("", "", bytes.NewBuffer(message))
			if err != nil {
				continue
			}
			err = FilterResponse(req, conf.Filters)
			if err != nil {
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(time.Second * 2))
			message, err = io.ReadAll(req.Body)
			if err != nil {
				continue
			}
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				continue
			}
		}
	}()
}

func HttpProxy(r *http.Request, backend url.URL) (*http.Response, error) {
	req, err := handlers.CloneRequest(r, handlers.WithUrl(backend))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)

}

func HandlerFunc(r *http.Request, filter handlers.Filter) error {
	res, err := filter.Handle(r)
	if err != nil {
		return NewHandlerError(HANDLER_ERROR_INTERNAL, res.StatusCode, err.Error())
	}
	if StatusCodeClass(res.StatusCode) != STATUS_SUCCESS {
		return NewHandlerError(HANDLER_ERROR_FILTER, res.StatusCode, res.Status)
	}
	err = filter.MoveTo(res, r)
	if err != nil {
		return err
	}
	return nil
}

func FilterRequest(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if !filter.Is(handlers.INTERCEPT) {
			continue
		}
		if !filter.Is(handlers.PARALLEL) {
			err := HandlerFunc(r, filter)
			if err != nil {
				return err
			}
			return nil
		}
		go HandlerFunc(r, filter)
	}
	return nil
}

func FilterResponse(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if !filter.Is(handlers.POST_PROCESS) {
			continue
		}
		if !filter.Is(handlers.PARALLEL) {
			err := HandlerFunc(r, filter)
			if err != nil {
				return err
			}
			return nil
		}
		go HandlerFunc(r, filter)
	}
	return nil
}

func Success(statusCode int) StatusCodeClass {
	if statusCode < 200 {
		return STATUS_NONE
	}
	if statusCode < 400 {
		return STATUS_SUCCESS
	}
	if statusCode < 500 {
		return STATUS_CLIENT_ERROR
	}
	return STATUS_SERVER_ERROR
}

func main() {
	frontend, err := url.Parse("/comms")
	_ = err
	backend, err := url.Parse("ws://127.0.0.1:3000/comms")
	mux := http.NewServeMux()
	handler := New(handlers.Conf{
		Frontend: *frontend,
		Backend:  *backend,
	})
	handler(mux)
	http.ListenAndServe(":8081", mux)
}
