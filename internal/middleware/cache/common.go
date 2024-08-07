package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	Cache struct {
		Address     *url.URL
		KeyTemplate string
		TTL         time.Duration
	}
	Response struct {
		Header http.Header
		Body   []byte
	}
)

func init() {
	gob.Register(http.Header{})
	gob.Register(Response{})
}

func Marshal(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	buffer := new(bytes.Buffer)
	if err != nil {
		return nil, err
	}
	err = gob.NewEncoder(buffer).Encode(Response{
		Header: r.Header,
		Body:   body,
	})
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func Unmarshal(data []byte) (*http.Response, error) {
	val := new(Response)
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(val)
	if err != nil {
		return nil, err
	}
	res := http.Response{}
	res.Header = val.Header.Clone()
	res.Body = io.NopCloser(bytes.NewReader(val.Body))
	return &res, nil
}

func (c *Cache) ParseKey(r *http.Request, rv netio.RouteValues) (string, error) {
	cacheKey := strings.ToLower(c.KeyTemplate)
	for key, value := range rv {
		cacheKey = strings.ReplaceAll(cacheKey, fmt.Sprintf("{:%s}", strings.ToLower(key)), value)
	}
	for key, value := range r.URL.Query() {
		cacheKey = strings.ReplaceAll(cacheKey, fmt.Sprintf("{?%s}", strings.ToLower(key)), strings.Join(value, "-"))
	}
	if strings.Contains(cacheKey, "{body}") {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		sha256 := sha256.New()
		_, err = sha256.Write(data)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum(nil)
		cacheKey = strings.ReplaceAll(cacheKey, "{body}", hex.EncodeToString(hash))
	}
	if strings.Contains(cacheKey, "{method}") {
		cacheKey = strings.ReplaceAll(cacheKey, "{body}", r.Method)
	}
	return cacheKey, nil
}

func (c *Cache) GetRequestUpdaters() []netio.RequestUpdater {
	return nil
}

func (c *Cache) GetResponseUpdaters() []netio.ResponseUpdater {
	return nil
}

func (c *Cache) OverrideRequestUpdaters([]netio.RequestUpdater) {

}

func (c *Cache) OverrideResponseUpdaters([]netio.ResponseUpdater) {

}

func (c *Cache) GetName() string {
	return "Cache"
}

func (c *Cache) GetAwaitList() []string {
	return nil
}

func (c *Cache) GetIsParallel() bool {
	return false
}

func (f *Cache) GetContext() context.Context {
	return context.TODO()
}

func (f *Cache) Build() ([]netio.Caller, error) {
	switch strings.ToLower(f.Address.Scheme) {
	case "jetstream":
		{
			jetstream, err := NewJetStream(f)
			if err != nil {
				return nil, err
			}
			return []netio.Caller{jetstream.Get(), jetstream.Set()}, nil
		}
	}
	return nil, fmt.Errorf("unsupported scheme %s", f.Address.Scheme)
}
