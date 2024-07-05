package proxies

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	WebSocketProxy struct {
		*Proxy
		ConnectCallers []netio.Caller

		ws *websocket.Conn

		listening bool
	}

	WebSocketReaderProxy struct {
		*WebSocketProxy
		Callers []netio.Caller
	}

	WebSocketWriterProxy struct {
		*WebSocketProxy
		Callers []netio.Caller
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
	for _, caller := range p.Callers {
		if caller.GetLevel() == netio.LEVEL_CONNECT {
			webSocketProxy.ConnectCallers = append(webSocketProxy.ConnectCallers, caller)
		}
	}
	return webSocketProxy
}

func NewWebSocketReaderProxy(webSocketProxy *WebSocketProxy) *WebSocketReaderProxy {
	webSocketReaderProxy := new(WebSocketReaderProxy)
	webSocketReaderProxy.WebSocketProxy = webSocketProxy
	webSocketReaderProxy.Callers = make([]netio.Caller, 0)
	for _, caller := range webSocketProxy.Callers {
		if caller.GetLevel() == netio.LEVEL_REQUEST {
			webSocketReaderProxy.Callers = append(webSocketReaderProxy.Callers, caller)
		}
	}
	return webSocketReaderProxy
}

func NewWebSocketWriterProxy(webSocketProxy *WebSocketProxy) *WebSocketWriterProxy {
	webSocketWriterProxy := new(WebSocketWriterProxy)
	webSocketWriterProxy.WebSocketProxy = webSocketProxy
	webSocketWriterProxy.Callers = make([]netio.Caller, 0)
	for _, caller := range webSocketProxy.Callers {
		if caller.GetLevel() == netio.LEVEL_RESPONSE {
			webSocketWriterProxy.Callers = append(webSocketWriterProxy.Callers, caller)
		}
	}
	return webSocketWriterProxy
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

func (f *WebSocketReaderProxy) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	t, sock, err := f.ws.NextReader()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	data, err := io.ReadAll(sock)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	res := new(http.Response)
	res.Body = io.NopCloser(bytes.NewReader(data))
	res.Header = http.Header{
		"Message-Type": []string{fmt.Sprintf("%d", t)},
	}
	return netio.TERM, res, nil
}

func (f *WebSocketWriterProxy) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	r, err := c()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	t, err := strconv.Atoi(r.Header.Get("Message-Type"))
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	err = f.ws.WriteMessage(t, data)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	return netio.TERM, nil, nil
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
	inProxy.ws = in
	inProxy.listening = true
	out, _, err := websocket.DefaultDialer.Dial(inProxy.Address.String(), r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	inReader := NewWebSocketReaderProxy(inProxy)
	inWriter := NewWebSocketWriterProxy(inProxy)

	outProxy := &WebSocketProxy{
		ws:        out,
		listening: true,
	}
	in.SetCloseHandler(func(code int, text string) error {
		inProxy.listening = false
		outProxy.listening = false
		return nil
	})
	outReader := NewWebSocketReaderProxy(outProxy)
	outWriter := NewWebSocketWriterProxy(outProxy)

	inReader.Callers = append([]netio.Caller{inReader}, inReader.Callers...)
	inReader.Callers = append(inReader.Callers, outWriter)

	outReader.Callers = append([]netio.Caller{outReader}, inWriter.Callers...)
	outReader.Callers = append(outReader.Callers, inWriter)

	go func() {
		for inProxy.listening {
			_, _ = netio.Cascade(req, inReader.Callers...)
		}
	}()
	go func() {
		for outProxy.listening {
			_, _ = netio.Cascade(req, outReader.Callers...)
		}
	}()
}
