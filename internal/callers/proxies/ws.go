package proxies

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	WebSocketProxy struct {
		*Proxy
		ConnectCallers  []netio.Caller
		RequestCallers  []netio.Caller
		ResponseCallers []netio.Caller
	}
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func NewWebSocket(p *Proxy) *WebSocketProxy {
	webSocketProxy := new(WebSocketProxy)
	webSocketProxy.Proxy = p
	webSocketProxy.ConnectCallers = make([]netio.Caller, 0)
	webSocketProxy.RequestCallers = make([]netio.Caller, 0)
	webSocketProxy.ResponseCallers = make([]netio.Caller, 0)
	for _, caller := range p.Callers {
		switch caller.GetLevel() {
		case netio.LEVEL_CONNECT:
			{
				webSocketProxy.ConnectCallers = append(webSocketProxy.ConnectCallers, caller)
			}
		case netio.LEVEL_REQUEST:
			{
				webSocketProxy.RequestCallers = append(webSocketProxy.RequestCallers, caller)
			}
		case netio.LEVEL_RESPONSE:
			{
				requestUpdaters := make([]netio.RequestUpdater, 0)
				for _, updater := range caller.GetResponseUpdaters() {
					fn := func(r *netio.ShadowRequest, httpR *http.Request) error {
						res, err := netio.NewShandowResponse(&http.Response{
							Header:  r.Header,
							Body:    r.Body,
							Trailer: r.Trailer,
						})
						if err != nil {
							return err
						}
						httpRes := http.Response{
							Header:  httpR.Header,
							Body:    httpR.Body,
							Trailer: httpR.Trailer,
						}
						return updater(res, &httpRes)
					}
					requestUpdaters = append(requestUpdaters, fn)
				}
				caller.OverrideRequestUpdaters(append(caller.GetRequestUpdaters(), requestUpdaters...))
				caller.OverrideResponseUpdaters(nil)
				webSocketProxy.ResponseCallers = append(webSocketProxy.ResponseCallers, caller)
			}
		}
	}
	return webSocketProxy
}

func (f *WebSocketProxy) GetRequestUpdaters() []netio.RequestUpdater {
	return nil
}

func (f *WebSocketProxy) GetResponseUpdaters() []netio.ResponseUpdater {
	return nil
}

func (f *WebSocketProxy) GetName() string {
	return f.Name
}

func (f *WebSocketProxy) GetAwaitList() []string {
	return nil
}

func (f *WebSocketProxy) GetIsParallel() bool {
	return false
}

func (f *WebSocketProxy) GetContext() context.Context {
	ctx, cancel := context.WithCancel(context.TODO())
	timeout := f.Timeout
	if timeout == 0 {
		timeout = time.Second * 30
	}
	time.AfterFunc(time.Until(time.Now().Add(timeout)), func() {
		cancel()
	})
	return ctx
}

func (f *WebSocketProxy) GetLevel() netio.Level {
	return netio.LEVEL_NONE
}

func (inProxy *WebSocketProxy) Handle(w http.ResponseWriter, r *http.Request, rv netio.RouteValues) {
	req, err := netio.NewShadowRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	_, _err := netio.Cascade(req, inProxy.ConnectCallers...)
	if _err != nil {
		http.Error(w, _err.Message(), _err.Status())
	}
	in, err := upgrader.Upgrade(w, r, http.Header{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	inProxy.Address.Path = r.URL.Path

	out, _, err := websocket.DefaultDialer.Dial(inProxy.Address.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	go func() {
		for {
			_, message, err := in.ReadMessage()
			if err != nil {
				break
			}
			httpReq, err := http.NewRequest("*", "", bytes.NewBuffer(message))
			if err != nil {
				continue
			}
			req, err := netio.NewShadowRequest(httpReq)
			if err != nil {
				continue
			}
			_, _err := netio.Cascade(req, inProxy.RequestCallers...)
			if _err != nil {
				continue
			}
			message, err = io.ReadAll(req.Body)
			if err != nil {
				continue
			}
			out.WriteMessage(websocket.TextMessage, message)
		}
	}()

	go func() {
		for {
			_, message, err := out.ReadMessage()
			if err != nil {
				break
			}
			httpReq, err := http.NewRequest("*", "", bytes.NewBuffer(message))
			if err != nil {
				continue
			}
			req, err := netio.NewShadowRequest(httpReq)
			if err != nil {
				continue
			}

			_, _err := netio.Cascade(req, inProxy.ResponseCallers...)
			if _err != nil {
				continue
			}
			message, err = io.ReadAll(req.Body)
			if err != nil {
				continue
			}
			in.WriteMessage(websocket.TextMessage, message)
		}
	}()
}
