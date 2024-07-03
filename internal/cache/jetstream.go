package cache

import (
	"context"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	JetStream struct {
		*Cache
		Host   string
		Bucket string
		kv     nats.KeyValue
	}
	JetStreamGet struct {
		*JetStream
	}
	JetStreamSet struct {
		*JetStream
	}
)

var (
	_conns   map[string]*nats.Conn
	_connMut sync.RWMutex
)

func init() {
	_conns = make(map[string]*nats.Conn)
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

func (f *JetStreamGet) Call(ctx context.Context, _ netio.Cloner, o netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	req, err := o()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	key, err := f.ParseKey(req)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	value, err := f.kv.Get(key)
	if err != nil {
		if err == nats.ErrKeyNotFound {
			return netio.CONTINUE, nil, nil
		}
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	res, err := Unmarshal(value.Value())
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	return netio.TERM, res, nil
}

func (f *JetStreamSet) Call(ctx context.Context, c netio.Cloner, o netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	req, err := o()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	key, err := f.ParseKey(req)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	res, err := c()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	data, err := Marshal(res)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	_, err = f.kv.Put(key, data)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	return netio.CONTINUE, nil, nil
}
