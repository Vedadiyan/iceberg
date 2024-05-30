package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/parsers"
)

type (
	StatusCodeClass int

	WebSocketProxy struct {
		Conf            *filters.Conf
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
		req, err := filters.CloneRequest(wsp.Request)
		if err != nil {
			wsp.Conn.WriteJSON(common.HandlerError{
				Class:      common.HANDLER_ERROR_INTERNAL,
				StatusCode: 500,
				Message:    err.Error(),
			})
			return
		}
		req.Body = io.NopCloser(bytes.NewBuffer(message))
		err = filters.HandleFilter(req, wsp.Conf.Filters, filters.REQUEST)
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
		err = filters.HandleFilter(req, wsp.Conf.Filters, filters.RESPONSE)
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

func HttpProxy(r *http.Request, backend *url.URL) (*http.Response, error) {
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

func NewWebSocketProxy(conf *filters.Conf, w http.ResponseWriter, r *http.Request) (*WebSocketProxy, error) {
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

func HandlerFunc(conf *filters.Conf) common.Handler {
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

func HandleCORS(conf *filters.Conf, w http.ResponseWriter, r *http.Request) bool {
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
	r, err = filters.RequestFrom(HttpProxy(r, conf.Backend))
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

func WebSocketHandler(conf *filters.Conf, w http.ResponseWriter, r *http.Request) {
	err := filters.HandleFilter(r, conf.Filters, filters.CONNECT)
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
			server, err = parsers.BuildV1(specV1, HandlerFunc)
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
