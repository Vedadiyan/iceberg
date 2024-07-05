package proxies

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	WebSocket struct {
		Name    string
		Address *url.URL
		Timeout time.Duration

		ResponseCaller []netio.Caller
		AwaitList      []string

		RequestUpdaters  []netio.RequestUpdater
		ResponseUpdaters []netio.ResponseUpdater

		in  *websocket.Conn
		out *websocket.Conn
	}

	WebSocketReader struct {
		*WebSocket
		Callers []netio.Caller
	}

	WebSocketWriter struct {
		*WebSocket
		Callers []netio.Caller
	}
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func NewWebSocket(address *url.URL, callers []netio.Caller) *WebSocket {
	httpProxy := new(WebSocket)
	httpProxy.Address = address
	httpProxy.ResponseUpdaters = make([]netio.ResponseUpdater, 0)
	httpProxy.RequestUpdaters = make([]netio.RequestUpdater, 0)
	httpProxy.RequestUpdaters = append(httpProxy.RequestUpdaters, netio.ReqReplaceBody(), netio.ReqReplaceHeader(), netio.ReqReplaceTailer())
	httpProxy.ResponseUpdaters = append(httpProxy.ResponseUpdaters, netio.ResReplaceBody(), netio.ResReplaceHeader(), netio.ResReplaceTailer())
	return httpProxy
}

func (f *WebSocket) GetRequestUpdaters() []netio.RequestUpdater {
	return f.RequestUpdaters
}

func (f *WebSocket) GetResponseUpdaters() []netio.ResponseUpdater {
	return f.ResponseUpdaters
}

func (f *WebSocket) GetName() string {
	return f.Name
}

func (f *WebSocket) GetAwaitList() []string {
	return f.AwaitList
}

func (f *WebSocket) GetIsParallel() bool {
	return false
}

func (f *WebSocket) GetContext() context.Context {
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

func (f *WebSocket) GetLevel() netio.Level {
	return netio.LEVEL_NONE
}

func (f *WebSocketReader) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	for {
		t, sock, err := f.in.NextReader()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(sock)
		if err != nil {
			continue
		}
		clone, err := c()
		if err != nil {
			continue
		}
		clone.Body = io.NopCloser(bytes.NewReader(data))
		in, err := netio.NewShadowRequest(clone)
		if err != nil {
			continue
		}
		res, _err := netio.Cascade(in, f.Callers...)
		if _err != nil {
			continue
		}
		data, err = io.ReadAll(res.Body)
		if err != nil {
			continue
		}
		f.out.WriteMessage(t, data)
	}
}

func (f *WebSocketWriter) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	for {
		t, sock, err := f.out.NextReader()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(sock)
		if err != nil {
			continue
		}

		clone, err := c()
		if err != nil {
			continue
		}
		clone.Body = io.NopCloser(bytes.NewReader(data))
		in, err := netio.NewShadowRequest(clone)
		if err != nil {
			continue
		}
		res, _err := netio.Cascade(in, f.ResponseCaller...)
		if _err != nil {
			continue
		}
		data, err = io.ReadAll(res.Body)
		if err != nil {
			continue
		}
		f.in.WriteMessage(t, data)
	}
}

func (f *WebSocket) Handle(w http.ResponseWriter, r *http.Request, rv netio.RouteValues) {
	reader := new(WebSocketReader)
	writer := new(WebSocketWriter)
	in, err := netio.NewShadowRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	go reader.Call(context.TODO(), nil, in.CloneRequest, nil)
	go writer.Call(context.TODO(), nil, in.CloneRequest, nil)
}
