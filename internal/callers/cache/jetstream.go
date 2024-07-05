package cache

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	JetStream struct {
		*Cache
		Host   string
		Bucket string
		kv     nats.KeyValue
		get    *JetStreamGet
		set    *JetStreamSet
	}
	JetStreamGet struct {
		*JetStream
	}
	JetStreamSet struct {
		*JetStream
	}
)

var (
	_kvs   map[string]nats.KeyValue
	_kvMut sync.RWMutex
)

func init() {
	_kvs = make(map[string]nats.KeyValue)
}

func GetKV(url string, bucket string, ttl time.Duration) (nats.KeyValue, error) {
	_kvMut.Lock()
	defer _kvMut.Unlock()
	if conn, ok := _kvs[url]; ok {
		return conn, nil
	}
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucket,
		TTL:    ttl,
	})
	if err != nil && err != jetstream.ErrBucketExists {
		return nil, err
	}
	return kv, nil
}

func NewJetStream(c *Cache) (*JetStream, error) {
	host := c.Address.Host
	if strings.HasPrefix(host, "[[") && strings.HasSuffix(host, "]]") {
		host = strings.TrimLeft(host, "[")
		host = strings.TrimRight(host, "]")
		host = os.Getenv(host)
	}
	jetstream := new(JetStream)
	jetstream.Cache = c
	jetstream.Host = host
	jetstream.Bucket = strings.TrimPrefix(c.Address.Path, "/")
	jetstream.get = NewJetStreamGet(jetstream)
	jetstream.set = NewJetStreamSet(jetstream)
	kv, err := GetKV(jetstream.Host, jetstream.Bucket, jetstream.TTL)
	if err != nil {
		return nil, err
	}
	jetstream.kv = kv
	return jetstream, nil
}

func NewJetStreamGet(js *JetStream) *JetStreamGet {
	jetstreamGet := new(JetStreamGet)
	jetstreamGet.JetStream = js
	return jetstreamGet
}

func NewJetStreamSet(js *JetStream) *JetStreamSet {
	jetstreamSet := new(JetStreamSet)
	jetstreamSet.JetStream = js
	return jetstreamSet
}

func (jetStream *JetStream) Get() netio.Caller {
	return jetStream.get
}

func (jetStream *JetStream) Set() netio.Caller {
	return jetStream.set
}

func (f *JetStreamGet) GetLevel() netio.Level {
	return netio.LEVEL_PRE
}

func (f *JetStreamGet) Call(ctx context.Context, rv netio.RouteValues, _ netio.Cloner, o netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	req, err := o()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	key, err := f.ParseKey(req, rv)
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

func (f *JetStreamSet) GetLevel() netio.Level {
	return netio.LEVEL_POST
}

func (f *JetStreamSet) Call(ctx context.Context, rv netio.RouteValues, c netio.Cloner, o netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	req, err := o()
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	key, err := f.ParseKey(req, rv)
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
