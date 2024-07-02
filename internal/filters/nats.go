package filters

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/internal/netio"
	natshelpers "github.com/vedadiyan/nats-helpers"
	queue "github.com/vedadiyan/nats-helpers/queue"
)

type (
	DurableNATSFilter struct {
		*Filter
		Host    string
		Subject string
		queue   *queue.Queue
		conn    *nats.Conn
	}
)

const (
	DURABLE_CHANNEL = "$ICEBERG.DURABLE"
)

var (
	_conns   map[string]*nats.Conn
	_connMut sync.RWMutex
)

func init() {
	_conns = make(map[string]*nats.Conn)
}

func GetConn(url string) (*nats.Conn, error) {
	_connMut.Lock()
	defer _connMut.Unlock()
	if conn, ok := _conns[url]; ok {
		return conn, nil
	}
	conn, err := nats.Connect(url)
	if err != nil {
		panic(err)
	}
	_ = conn
	panic("")
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

func NewDurableNATSFilter(f *Filter) (*DurableNATSFilter, error) {
	conn, err := GetConn("")
	if err != nil {
		return nil, err
	}
	queue, err := queue.New(conn, []string{})
	if err != nil {
		return nil, err
	}
	queue.Pull(DURABLE_CHANNEL, func(m *nats.Msg) natshelpers.State {
		defer func() {
			go func() {
				shadowRequest, err := MsgToRequest(m)
				if err != nil {
					log.Println(err)
				}
				netio.Cascade(shadowRequest, f.Callers...)
			}()
		}()
		clone := *m
		clone.Subject = clone.Reply
		clone.Reply = ""
		err := conn.PublishMsg(&clone)
		_ = err
		return natshelpers.Done()
	})
	natsFilter := new(DurableNATSFilter)
	natsFilter.Filter = f
	natsFilter.conn = conn
	natsFilter.queue = queue
	f.instance = natsFilter
	return natsFilter, nil
}

func (durableNATSFilter *DurableNATSFilter) Call(ctx context.Context, c netio.Cloner) (bool, *http.Response, error) {
	var (
		res *netio.ShadowResponse
		err error
	)
	inbox := durableNATSFilter.conn.NewRespInbox()
	var wg sync.WaitGroup
	subs, err := durableNATSFilter.conn.Subscribe(inbox, func(msg *nats.Msg) {
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
	err = durableNATSFilter.queue.PushMsg(&nats.Msg{})
	if err != nil {
		return false, nil, err
	}
	wg.Wait()
	return false, res.Response, err
}
