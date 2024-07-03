package filters

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/internal/netio"
	natshelpers "github.com/vedadiyan/nats-helpers"
	"github.com/vedadiyan/nats-helpers/headers"
	queue "github.com/vedadiyan/nats-helpers/queue"
)

type (
	NatsBase struct {
		*Filter
		Host    string
		Subject string
		conn    *nats.Conn
	}
	NatsJSFilter struct {
		*NatsBase
		queue *queue.Queue
	}

	NatsCoreFilter struct {
		*NatsBase
	}
)

const (
	DURABLE_CHANNEL = "$ICEBERG.DURABLE"
)

var (
	_conns   map[string]*nats.Conn
	_gc      []func() error
	_connMut sync.RWMutex
)

func init() {
	_conns = make(map[string]*nats.Conn)
	_gc = make([]func() error, 0)
}

func GetConn(url string, fn func(*nats.Conn) error) (*nats.Conn, error) {
	_connMut.Lock()
	defer _connMut.Unlock()
	if conn, ok := _conns[url]; ok {
		return conn, nil
	}
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	if fn != nil {
		err := fn(conn)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func MsgToRequest(m *nats.Msg) (*netio.ShadowRequest, error) {
	req := http.Request{
		Header: http.Header{},
	}
	for key, values := range m.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(m.Data))
	return netio.NewShadowRequest(&req)
}

func MsgToResponse(m *nats.Msg) (*netio.ShadowResponse, error) {
	res := http.Response{
		Header: http.Header{},
	}
	for key, values := range m.Header {
		for _, value := range values {
			res.Header.Add(key, value)
		}
	}
	res.Body = io.NopCloser(bytes.NewReader(m.Data))
	return netio.NewShandowResponse(&res)
}

func NewBaseNATS(f *Filter) *NatsBase {
	host := f.Address.Host
	if strings.HasPrefix(host, "[[") && strings.HasSuffix(host, "]]") {
		host = strings.TrimLeft(host, "[")
		host = strings.TrimRight(host, "]")
		host = os.Getenv(host)
	}
	baseNATS := new(NatsBase)
	baseNATS.Filter = f
	baseNATS.Host = host
	baseNATS.Subject = f.Address.Path
	return baseNATS
}

func NewDurableNATSFilter(f *NatsBase) (*NatsJSFilter, error) {
	conn, err := GetConn(f.Host, CreateReflectorChannel(f))
	if err != nil {
		return nil, err
	}
	queue, err := queue.New(conn, []string{f.Subject})
	if err != nil {
		return nil, err
	}
	nf := new(NatsJSFilter)
	nf.NatsBase = f
	nf.conn = conn
	nf.queue = queue
	f.instance = nf
	return nf, nil
}

func (f *NatsJSFilter) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	inbox := f.conn.NewRespInbox()
	resCh := make(chan *netio.ShadowResponse, 1)
	errCh := make(chan error, 1)
	defer close(resCh)
	defer close(errCh)
	err := f.SubscribeOnce(inbox, resCh, errCh)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	err = f.Publish(inbox, c)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusBadGateway)
	}
	return Await(resCh, errCh, ctx)
}

func (f *NatsJSFilter) SubscribeOnce(inbox string, resCh chan<- *netio.ShadowResponse, errCh chan<- error) error {
	handle := func(msg *nats.Msg) {
		res, err := MsgToResponse(msg)
		if err != nil {
			errCh <- err
			return
		}
		resCh <- res
	}
	subs, err := f.conn.Subscribe(inbox, func(msg *nats.Msg) {
		go handle(msg)
	})
	if err != nil {
		return err
	}
	return subs.AutoUnsubscribe(1)
}

func (f *NatsJSFilter) Publish(inbox string, c netio.Cloner) error {
	req, err := c()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	msg := &nats.Msg{
		Subject: f.Subject,
		Header:  nats.Header{},
		Data:    data,
	}
	hdr := headers.Header(req.Header.Clone())
	hdr.SetReflector(DURABLE_CHANNEL)
	hdr.SetReply(inbox)
	err = hdr.Export(msg.Header)
	if err != nil {
		return err
	}
	return f.queue.PushMsg(msg)
}

func NewCoreNATSFilter(f *NatsBase) (*NatsCoreFilter, error) {
	conn, err := GetConn(f.Host, nil)
	if err != nil {
		return nil, err
	}
	nf := new(NatsCoreFilter)
	nf.NatsBase = f
	nf.conn = conn
	f.instance = nf
	return nf, nil
}

func (f *NatsCoreFilter) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	inbox := f.conn.NewRespInbox()
	resCh := make(chan *netio.ShadowResponse, 1)
	errCh := make(chan error, 1)
	defer close(resCh)
	defer close(errCh)
	err := f.SubscribeOnce(inbox, resCh, errCh)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	err = f.Publish(inbox, c)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusBadGateway)
	}
	return Await(resCh, errCh, ctx)
}

func (f *NatsCoreFilter) SubscribeOnce(inbox string, resCh chan<- *netio.ShadowResponse, errCh chan<- error) error {
	handle := func(msg *nats.Msg) {
		clone := *msg
		headers, err := headers.Import(clone.Header)
		if err != nil {
			errCh <- err
			return
		}
		clone.Header = nats.Header(headers)
		res, err := MsgToResponse(msg)
		if err != nil {
			errCh <- err
			return
		}
		resCh <- res
	}
	callbacks := func(msg *nats.Msg) {
		shadowRequest, err := MsgToRequest(msg)
		if err != nil {
			log.Println(err)
		}
		netio.Cascade(shadowRequest, f.Callers...)
	}
	subs, err := f.conn.Subscribe(inbox, func(msg *nats.Msg) {
		go func() {
			defer callbacks(msg)
			handle(msg)
		}()
	})
	if err != nil {
		return err
	}
	return subs.AutoUnsubscribe(1)
}

func (f *NatsCoreFilter) Publish(inbox string, c netio.Cloner) error {
	req, err := c()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return f.conn.PublishMsg(&nats.Msg{
		Subject: f.Subject,
		Reply:   inbox,
		Header:  nats.Header(req.Header.Clone()),
		Data:    data,
	})
}

func CreateReflectorChannel(f *NatsBase) func(c *nats.Conn) error {
	return func(c *nats.Conn) error {
		queue, err := queue.New(c, []string{DURABLE_CHANNEL})
		if err != nil {
			return err
		}
		handle := func(m *nats.Msg) error {
			clone := *m
			headers, err := headers.Import(clone.Header)
			if err != nil {
				return err
			}
			reply := headers.GetReply()
			if len(reply) == 0 {
				return err
			}
			m.Subject = reply
			headers.DeleteReply()
			err = headers.Export(clone.Header)
			if err != nil {
				return err
			}
			return c.PublishMsg(&clone)
		}
		callbacks := func(msg *nats.Msg) {
			shadowRequest, err := MsgToRequest(msg)
			if err != nil {
				log.Println(err)
			}
			netio.Cascade(shadowRequest, f.Callers...)
		}
		stop, err := queue.Pull(DURABLE_CHANNEL, func(msg *nats.Msg) natshelpers.State {
			defer callbacks(msg)
			err := handle(msg)
			if err != nil {
				return natshelpers.Drop()
			}
			return natshelpers.Done()
		})
		_gc = append(_gc, stop)
		return err
	}
}
