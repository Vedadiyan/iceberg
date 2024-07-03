package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vedadiyan/iceberg/internal/router"
)

type (
	Cache struct {
		Route       *router.Route
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

func (c *Cache) ParseKey(r *http.Request) (string, error) {
	routeValues := c.Route.Bind(router.ParseRoute(r.URL, "*"))
	cacheKey := strings.ToLower(c.KeyTemplate)
	for key, value := range routeValues {
		cacheKey = strings.ReplaceAll(cacheKey, fmt.Sprintf("${:%s}", strings.ToLower(key)), value)
	}
	for key, value := range r.URL.Query() {
		cacheKey = strings.ReplaceAll(cacheKey, fmt.Sprintf("${?%s}", strings.ToLower(key)), strings.Join(value, "-"))
	}
	if strings.Contains(cacheKey, "${body}") {
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
		cacheKey = strings.ReplaceAll(cacheKey, "${body}", hex.EncodeToString(hash))
	}
	return cacheKey, nil
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
