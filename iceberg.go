package main

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/handlers"
)

type (
	StatusCodeClass int
	Handler         func(*http.ServeMux)
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

const (
	NONE         StatusCodeClass = 0
	SUCCESS      StatusCodeClass = 1
	CLIENT_ERROR StatusCodeClass = 2
	SERVER_ERROR StatusCodeClass = 3
)

func New(conf handlers.Conf) Handler {
	return func(sm *http.ServeMux) {
		sm.HandleFunc(conf.Frontend.String(), func(w http.ResponseWriter, r *http.Request) {
			if IsWebSocket(r) {
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
	proxiedConn, _, err := websocket.DefaultDialer.Dial(conf.Backend.String(), r.Header)
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
			conn.SetReadDeadline(time.Now().Add(time.Second * 2))
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return
				}
				continue
			}
			req, err := handlers.CloneRequest(r)
			if err != nil {
				return
			}
			req.Body = io.NopCloser(bytes.NewBuffer(data))
			err = FilterSocketRequest(req, conf.Filters)
			if err != nil {
				return
			}
			proxiedConn.SetWriteDeadline(time.Now().Add(time.Second * 2))
			err = proxiedConn.WriteMessage(messageType, data)
			if err != nil {
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
			proxiedConn.SetReadDeadline(time.Now().Add(time.Second * 2))
			messageType, data, err := proxiedConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return
				}
				continue
			}
			err = FilterSocketResponse(nil, conf.Filters)
			if err != nil {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(time.Second * 2))
			err = conn.WriteMessage(messageType, data)
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
		return err
	}
	if StatusCodeClass(res.StatusCode) != SUCCESS {
		return err

	}
	err = filter.MoveTo(res, r)
	if err != nil {
		return err
	}
	return nil
}

func FilterRequest(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.INTERCEPT {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func FilterSocketRequest(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.INTERCEPT_SOCKET {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func FilterResponse(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.POST_PROCESS {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func FilterSocketResponse(r *http.Request, filters []handlers.Filter) error {
	for _, filter := range filters {
		if filter.Level() != handlers.POST_PROCESS_SOCKET {
			continue
		}
		err := HandlerFunc(r, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func Success(statusCode int) StatusCodeClass {
	if statusCode < 200 {
		return NONE
	}
	if statusCode < 400 {
		return SUCCESS
	}
	if statusCode < 500 {
		return CLIENT_ERROR
	}
	return SERVER_ERROR
}
