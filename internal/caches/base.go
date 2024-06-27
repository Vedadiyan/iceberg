package caches

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

type (
	Cache interface {
		Get(key string) ([]byte, error)
		Set(key string, value []byte) error
		Key(rv map[string]string, r *http.Request) (string, error)
	}
	KeyParams struct {
		BaseKey string
		Static  []string
		Route   []string
		Query   []string
		Body    bool
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

func (keyParams *KeyParams) GetKey(rv map[string]string, r *http.Request) (string, error) {
	buffer := bytes.NewBufferString(fmt.Sprintf("%s__%s__", keyParams.BaseKey, r.Method))

	for _, key := range keyParams.Static {
		buffer.WriteString(fmt.Sprintf("%s__", key))
	}

	for _, key := range keyParams.Route {
		value, ok := rv[key]
		if !ok {
			return "", fmt.Errorf("key not found")
		}
		buffer.WriteString(fmt.Sprintf("%s:%s__", key, value))
	}

	for _, key := range keyParams.Query {
		value := r.URL.Query().Get(key)
		if len(value) == 0 {
			continue
		}
		buffer.WriteString(fmt.Sprintf("%s:%s__", key, value))
	}

	if keyParams.Body {
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		buffer.WriteString("\r\n\r\n")
		buffer.Write(bytes)
	}

	sha256 := sha256.New()
	_, err := sha256.Write(buffer.Bytes())
	if err != nil {
		return "", nil
	}
	hash := sha256.Sum(nil)
	return hex.EncodeToString(hash), nil
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

func Unmarshal(data []byte) (*Response, error) {
	res := new(Response)
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
