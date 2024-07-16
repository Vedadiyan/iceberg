package opa

import (
	"encoding/json"
	"fmt"
	"io"
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
	PolicyType string
	OpaHttp    struct {
		*Opa
		Policies map[string]PolicyType
	}
	OpaHttpForNats struct {
		*OpaHttp
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

func NewOpaHttp(opa *Opa, policies map[string]PolicyType) *OpaHttp {
	opaHttp := new(OpaHttp)
	opaHttp.Opa = opa
	opaHttp.Policies = policies
	return opaHttp
}

func NewOpaHttpForNats(opaHttp *OpaHttp) (*OpaHttpForNats, error) {
	opaHttpForNats := new(OpaHttpForNats)
	opaHttpForNats.OpaHttp = opaHttp
	host := opaHttp.Agent.Host
	if strings.HasPrefix(host, "[[") && strings.HasSuffix(host, "]]") {
		host = strings.TrimLeft(host, "[")
		host = strings.TrimRight(host, "]")
		host = os.Getenv(host)
	}
	opaHttpForNats.Host = host
	opaHttpForNats.Subject = strings.TrimPrefix(opaHttp.Agent.Path, "/")
	conn, err := GetConn(host)
	if err != nil {
		return nil, err
	}
	opaHttpForNats.conn = conn
	kv, err := GetKV(conn, "OPA_STORE")
	if err != nil {
		return nil, err
	}
	policies := make([]string, 0)
	for p, t := range opaHttp.Policies {
		if t == POLICY_TYPE_LOCAL {
			name := fmt.Sprintf("%s_%s", opaHttp.Opa.AppName, p)
			policies = append(policies, name)
			_, err := kv.Put(name, []byte(os.Getenv(p)))
			if err != nil {
				return nil, err
			}
			continue
		}
		policies = append(policies, p)
	}
	opaHttpForNats.Policies = policies
	return opaHttpForNats, nil
}

func (opaHttpForNats *OpaHttpForNats) Eval(r *http.Request, rv netio.RouteValues) (bool, string, error) {
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
	res, err := opaHttpForNats.conn.RequestMsg(&nats.Msg{
		Subject: opaHttpForNats.Subject,
		Header: nats.Header{
			"X-Policies": opaHttpForNats.Policies,
		},
		Data: json,
	}, time.Second)
	if err != nil {
		return false, "", err
	}
	status := res.Header.Get("X-Status")
	if status != "200" {
		return false, res.Header.Get("X-Error"), nil
	}
	return true, "", nil
}
