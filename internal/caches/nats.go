package caches

import (
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type (
	JetStream struct {
		conn      *nats.Conn
		bucket    string
		ttl       time.Duration
		kv        nats.KeyValue
		KeyParams KeyParams
		once      sync.Once
	}
)

func (js *JetStream) Get(rv map[string]string, r *http.Request) ([]byte, error) {
	err := js.init()
	if err != nil {
		return nil, err
	}
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
	err := js.init()
	if err != nil {
		return err
	}
	key, err := js.KeyParams.GetKey(rv, r)
	if err != nil {
		return err
	}
	_, err = js.kv.Put(key, value)
	return err
}

func (js *JetStream) init() error {
	var err error
	js.once.Do(func() {
		_js, _err := js.conn.JetStream()
		if _err != nil {
			err = _err
			return
		}
		kv, _err := _js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: js.bucket,
			TTL:    js.ttl,
		})
		if _err == jetstream.ErrBucketExists {
			kv, _err = _js.KeyValue(js.bucket)
		}
		if _err != nil {
			err = _err
			return
		}
		js.kv = kv
	})
	return err
}
