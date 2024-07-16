package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	PolicyType string
	OpaNats    struct {
		*Opa
		Host     string
		Subject  string
		conn     *nats.Conn
		Policies []string
	}
)

var (
	_conns   map[string]*nats.Conn
	_connMut sync.RWMutex
)

const (
	POLICY_TYPE_LOCAL  PolicyType = "local"
	POLICY_TYPE_REMOTE PolicyType = "remote"
)

func GetConn(url string) (*nats.Conn, error) {
	_connMut.Lock()
	defer _connMut.Unlock()
	if conn, ok := _conns[url]; ok {
		return conn, nil
	}
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func GetKV(conn *nats.Conn, bucket string) (nats.KeyValue, error) {
	js, err := conn.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucket,
	})
	if err != nil && err != jetstream.ErrBucketExists {
		return nil, err
	}
	return kv, nil
}

func NewOpaNats(opa *Opa) (*OpaNats, error) {
	opaNats := new(OpaNats)
	opaNats.Opa = opa
	host := opa.Agent.Host
	if strings.HasPrefix(host, "[[") && strings.HasSuffix(host, "]]") {
		host = strings.TrimLeft(host, "[")
		host = strings.TrimRight(host, "]")
		host = os.Getenv(host)
	}
	opaNats.Host = host
	opaNats.Subject = strings.TrimPrefix(opa.Agent.Path, "/")
	conn, err := GetConn(host)
	if err != nil {
		return nil, err
	}
	opaNats.conn = conn
	kv, err := GetKV(conn, "OPA_STORE")
	if err != nil {
		return nil, err
	}
	policies := make([]string, 0)
	for p, t := range opa.Policies {
		if t == POLICY_TYPE_LOCAL {
			name := fmt.Sprintf("%s_%s", opa.AppName, p)
			policies = append(policies, name)
			_, err := kv.Put(name, []byte(os.Getenv(p)))
			if err != nil {
				return nil, err
			}
			continue
		}
		policies = append(policies, p)
	}
	opaNats.Policies = policies
	return opaNats, nil
}

func (opaNats *OpaNats) Eval(r *http.Request, rv netio.RouteValues) (bool, string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false, "", err
	}
	data := map[string]any{
		"path":    rv,
		"headers": r.Header,
		"method":  r.Method,
		"data":    body,
	}
	json, err := json.Marshal(data)
	if err != nil {
		return false, "", err
	}
	res, err := opaNats.conn.RequestMsg(&nats.Msg{
		Subject: opaNats.Subject,
		Header: nats.Header{
			"X-Policies": opaNats.Policies,
		},
		Data: json,
	}, opaNats.Opa.Timeout)
	if err != nil {
		return false, "", err
	}
	status := res.Header.Get("X-Status")
	if status != "200" {
		return false, res.Header.Get("X-Error"), nil
	}
	return true, "", nil
}

func (opaNats *OpaNats) Call(ctc context.Context, rv netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	r, err := c()
	if err != nil {
		return false, nil, netio.NewError(err.Error(), 500)
	}
	res, msg, err := opaNats.Eval(r, rv)
	if err != nil {
		return false, nil, netio.NewError(err.Error(), 500)
	}
	if !res {
		return false, nil, netio.NewError(msg, 400)
	}
	return true, nil, nil
}
