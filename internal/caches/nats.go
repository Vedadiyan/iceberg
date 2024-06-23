package caches

import (
	"net/http"

	"github.com/nats-io/nats.go"
)

type (
	JetStream struct {
		js        nats.KeyValue
		KeyParams KeyParams
	}
)

func (js *JetStream) Get(rv map[string]string, r *http.Request) ([]byte, error) {
	key, err := js.KeyParams.GetKey(rv, r)
	if err != nil {
		return nil, err
	}
	value, err := js.js.Get(key)
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
	_, err = js.js.Put(key, value)
	return err
}
