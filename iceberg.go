package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
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
	WebSocketProxy struct {
		Conf            *handlers.Conf
		Conn            *websocket.Conn
		ProxiedConn     *websocket.Conn
		Request         *http.Request
		interceptListen bool
		proxyListen     bool
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

func (wsp *WebSocketProxy) RequestHandler() {
	defer func() {
		wsp.Conn.Close()
		wsp.proxyListen = false
	}()
	for wsp.interceptListen {
		messageType, data, err := wsp.Conn.NextReader()
		if err != nil {
			return
		}
		message, err := io.ReadAll(data)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		req, err := handlers.CloneRequest(wsp.Request)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			return
		}
		req.Body = io.NopCloser(bytes.NewBuffer(message))
		err = Filter(req, wsp.Conf.Filters, handlers.REQUEST)
		if err != nil {
			if handlerError, ok := err.(HandlerError); ok {
				wsp.Conn.WriteJSON(handlerError)
				return
			}
			wsp.Conn.WriteJSON(HandlerError{
				StatusCode: 418,
			})
			return
		}
		err = wsp.ProxiedConn.WriteMessage(messageType, message)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_PROXY,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
	}
}

func (wsp *WebSocketProxy) ResponseHandler() {
	defer func() {
		wsp.ProxiedConn.Close()
		wsp.interceptListen = false
	}()
	for wsp.proxyListen {
		messageType, data, err := wsp.ProxiedConn.NextReader()
		if err != nil {
			return
		}
		message, err := io.ReadAll(data)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		req, err := http.NewRequest("", "", bytes.NewBuffer(message))
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		err = Filter(req, wsp.Conf.Filters, handlers.RESPONSE)
		if err != nil {
			if handlerError, ok := err.(HandlerError); ok {
				wsp.Conn.WriteJSON(handlerError)
				continue
			}
			wsp.Conn.WriteJSON(HandlerError{
				StatusCode: 418,
			})
			continue
		}
		message, err = io.ReadAll(req.Body)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		err = wsp.Conn.WriteMessage(messageType, message)
		if err != nil {
			wsp.Conn.WriteJSON(HandlerError{
				Class:      HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
	}
}

func NewHandlerError(class HandlerErrorClass, statusCode int, message string) error {
	handlerError := HandlerError{
		Class:   class,
		Message: message,
	}
	return handlerError
}

func NewWebSocketProxy(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) (*WebSocketProxy, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	proxiedConn, _, err := websocket.DefaultDialer.Dial(conf.Backend.String(), nil)
	if err != nil {
		return nil, err
	}
	webSocketProxy := WebSocketProxy{
		Conn:            conn,
		Conf:            conf,
		ProxiedConn:     proxiedConn,
		interceptListen: true,
		proxyListen:     true,
	}
	return &webSocketProxy, nil
}

func New(conf *handlers.Conf) Handler {
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

func HttpHandler(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) {
	err := Filter(r, conf.Filters, handlers.REQUEST)
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
		w.WriteHeader(502)
		return
	}
	err = Filter(r, conf.Filters, handlers.RESPONSE)
	if err != nil {
		if handlerError, ok := err.(HandlerError); ok {
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

func WebSocketHandler(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) {
	err := Filter(r, conf.Filters, handlers.CONNECT)
	if err != nil {
		if handlerError, ok := err.(HandlerError); ok {
			w.WriteHeader(handlerError.StatusCode)
			w.Write([]byte(handlerError.Message))
			return
		}
		w.WriteHeader(418)
		return
	}

	proxy, err := NewWebSocketProxy(conf, w, r)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	go proxy.RequestHandler()
	go proxy.ResponseHandler()
}

func HttpProxy(r *http.Request, backend *url.URL) (*http.Response, error) {
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

func Filter(r *http.Request, filters []handlers.Filter, level handlers.Level) error {
	for _, filter := range filters {
		if !filter.Is(level) {
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
	ver, conf, err := Parse()
	if err != nil {
		log.Fatalln(err.Error())
	}
	auto.ForConfigMap().Bootstrap()
	var server Server
	switch ver {
	case VER_V1:
		{
			specV1, ok := conf.(*SpecV1)
			if !ok {
				log.Fatalln("invalid program")
			}
			server, err = BuildV1(specV1)
			if err != nil {
				log.Fatalln(err.Error())
			}
		}
	default:
		{
			log.Fatalln("unsupported api version")
		}
	}
	err = server()
	if err != nil {
		log.Fatalln(err.Error())
	}
}
