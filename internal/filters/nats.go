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
	natsFilter := new(NatsJSFilter)
	natsFilter.NatsBase = f
	natsFilter.conn = conn
	natsFilter.queue = queue
	f.instance = natsFilter
	return natsFilter, nil
}

func (f *NatsJSFilter) Call(ctx context.Context, c netio.Cloner) (bool, *http.Response, error) {
	var (
		res *netio.ShadowResponse
		err error
	)
	inbox := f.conn.NewRespInbox()
	var wg sync.WaitGroup
	subs, err := f.conn.Subscribe(inbox, func(msg *nats.Msg) {
		go func() {
			defer wg.Done()
			res, err = MsgToResponse(msg)
		}()
	})
	if err != nil {
		return false, nil, err
	}
	err = subs.AutoUnsubscribe(1)
	if err != nil {
		return false, nil, err
	}
	req, err := c(netio.WithContext(ctx))
	if err != nil {
		return false, nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return false, nil, err
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
		return false, nil, err
	}
	err = f.queue.PushMsg(msg)
	if err != nil {
		return false, nil, err
	}
	wg.Wait()
	return false, res.Response, err
}

func NewCoreNATSFilter(f *NatsBase) (*NatsCoreFilter, error) {
	conn, err := GetConn(f.Host, nil)
	if err != nil {
		return nil, err
	}

	natsFilter := new(NatsCoreFilter)
	natsFilter.NatsBase = f
	natsFilter.conn = conn
	f.instance = natsFilter
	return natsFilter, nil
}

func (f *NatsCoreFilter) Call(ctx context.Context, c netio.Cloner) (bool, *http.Response, error) {
	var (
		res *netio.ShadowResponse
		err error
	)
	inbox := f.conn.NewRespInbox()
	var wg sync.WaitGroup
	subs, err := f.conn.Subscribe(inbox, func(m *nats.Msg) {
		go func() {
			defer func() {
				go func() {
					shadowRequest, err := MsgToRequest(m)
					if err != nil {
						log.Println(err)
					}
					netio.Cascade(shadowRequest, f.Callers...)
				}()
			}()
			defer wg.Done()
			res, err = MsgToResponse(m)
		}()
	})
	if err != nil {
		return false, nil, err
	}
	err = subs.AutoUnsubscribe(1)
	if err != nil {
		return false, nil, err
	}
	req, err := c(netio.WithContext(ctx))
	if err != nil {
		return false, nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return false, nil, err
	}
	err = f.conn.PublishMsg(&nats.Msg{
		Subject: f.Subject,
		Reply:   inbox,
		Header:  nats.Header(req.Header.Clone()),
		Data:    data,
	})
	if err != nil {
		return false, nil, err
	}
	wg.Wait()
	return false, res.Response, err
}

func CreateReflectorChannel(f *NatsBase) func(c *nats.Conn) error {
	return func(c *nats.Conn) error {
		queue, err := queue.New(c, []string{DURABLE_CHANNEL})
		if err != nil {
			return err
		}
		stop, err := queue.Pull(DURABLE_CHANNEL, func(m *nats.Msg) natshelpers.State {
			defer func() {
				go func() {
					shadowRequest, err := MsgToRequest(m)
					if err != nil {
						log.Println(err)
					}
					netio.Cascade(shadowRequest, f.Callers...)
				}()
			}()
			defer func() {
				go func() {
					clone := *m
					headers, err := headers.Import(clone.Header)
					_ = err
					reply := headers.GetReply()
					if len(reply) == 0 {
						return
					}
					m.Subject = reply
					headers.DeleteReply()
					headers.Export(clone.Header)
					err = c.PublishMsg(&clone)
					_ = err
				}()
			}()
			return natshelpers.Done()
		})
		_gc = append(_gc, stop)
		return err
	}

}
