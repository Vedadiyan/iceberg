package caches

import (
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vedadiyan/goal/pkg/di"
)

type (
	JetStream struct {
		Url       string
		Bucket    string
		TTL       time.Duration
		conn      *nats.Conn
		kv        nats.KeyValue
		KeyParams KeyParams
		once      sync.Once
	}
)

func (js *JetStream) Get(rv map[string]string, r *http.Request) ([]byte, error) {
	key, err := js.KeyParams.GetKey(rv, r)
	if err != nil {
		return nil, err
	}
	value, err := js.kv.Get(key)
	if err != nil {
		return nil, err
	}
	return value.Value(), nil
}

func (js *JetStream) Set(rv map[string]string, r *http.Request, value []byte) error {
	key, err := js.KeyParams.GetKey(rv, r)
	if err != nil {
		return err
	}
	_, err = js.kv.Put(key, value)
	return err
}

func (js *JetStream) Initializer() error {
	var err error
	js.once.Do(func() {
		conn, _err := di.ResolveWithName[nats.Conn](js.Url, nil)
		if err != nil {
			err = _err
			return
		}
		js.conn = conn
		_js, _err := js.conn.JetStream()
		if _err != nil {
			err = _err
			return
		}
		kv, _err := _js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: js.Bucket,
			TTL:    js.TTL,
		})
		if _err == jetstream.ErrBucketExists {
			kv, _err = _js.KeyValue(js.Bucket)
		}
		if _err != nil {
			err = _err
			return
		}
		js.kv = kv
	})
	return err
}
