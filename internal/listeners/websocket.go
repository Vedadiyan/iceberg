package listeners

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
)

type (
	WebSocketProxy struct {
		Conf            *filters.Conf
		Conn            *websocket.Conn
		ProxiedConn     *websocket.Conn
		Request         *http.Request
		interceptListen bool
		proxyListen     bool
	}
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func WebSocketHandler(conf *filters.Conf, w http.ResponseWriter, r *http.Request) {
	if HandleCORS(conf, w, r) {
		return
	}
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

func IsWebSocket(r *http.Request) bool {
	for _, value := range r.Header["Upgrade"] {
		if value == "websocket" {
			return true
		}
	}
	return false
}
