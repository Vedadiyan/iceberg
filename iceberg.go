package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/iceberg/common"
	"github.com/vedadiyan/iceberg/handlers"
)

type (
	StatusCodeClass int
	Handler         func(*http.ServeMux)

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
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

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
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		req, err := handlers.CloneRequest(wsp.Request)
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			return
		}
		req.Body = io.NopCloser(bytes.NewBuffer(message))
		err = handlers.HandleFilter(req, wsp.Conf.Filters, handlers.REQUEST)
		if err != nil {
			if handlerError, ok := err.(common.HandlerError); ok {
				wsp.Conn.WriteJSON(handlerError)
				return
			}
			wsp.Conn.WriteJSON(common.HandlerError{
				StatusCode: 418,
			})
			return
		}
		err = wsp.ProxiedConn.WriteMessage(messageType, message)
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_PROXY,
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
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		req, err := http.NewRequest("", "", bytes.NewBuffer(message))
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		err = handlers.HandleFilter(req, wsp.Conf.Filters, handlers.RESPONSE)
		if err != nil {
			if handlerError, ok := err.(common.HandlerError); ok {
				wsp.Conn.WriteJSON(handlerError)
				continue
			}
			wsp.Conn.WriteJSON(common.HandlerError{
				StatusCode: 418,
			})
			continue
		}
		message, err = io.ReadAll(req.Body)
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
		err = wsp.Conn.WriteMessage(messageType, message)
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			continue
		}
	}
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

func HandleCORS(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) bool {
	if conf.CORS != nil {
		if r.Method == "OPTIONS" {
			w.Header().Add("access-control-allow-origin", conf.CORS.Origins)
			w.Header().Add("access-control-allow-headers", conf.CORS.Headers)
			w.Header().Add("access-control-max-age", conf.CORS.MaxAge)
			w.Header().Add("access-control-allow-methods", conf.CORS.Methods)
			w.WriteHeader(200)
			return true
		}
		w.Header().Add("Access-Control-Expose-Headers", conf.CORS.ExposeHeader)
	}
	return false
}

func HttpHandler(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) {
	if HandleCORS(conf, w, r) {
		return
	}

	log.Println("handling request", r.URL.String(), r.Method)
	log.Println("handling request filters")
	err := handlers.HandleFilter(r, conf.Filters, handlers.REQUEST)
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
	r, err = handlers.RequestFrom(handlers.HttpProxy(r, conf.Backend))
	if err != nil {
		log.Println(err)
		w.WriteHeader(502)
		return
	}
	r.URL = &url
	log.Println("handling response filters")
	err = handlers.HandleFilter(r, conf.Filters, handlers.RESPONSE)
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

func WebSocketHandler(conf *handlers.Conf, w http.ResponseWriter, r *http.Request) {
	err := handlers.HandleFilter(r, conf.Filters, handlers.CONNECT)
	if err != nil {
		if handlerError, ok := err.(common.HandlerError); ok {
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
	auto.ForConfigMap().Bootstrap()
	err = server()
	if err != nil {
		log.Fatalln(err.Error())
	}
}
