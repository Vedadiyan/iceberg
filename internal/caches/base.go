package caches

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

type (
	Cache interface {
		Get(rv map[string]string, r *http.Request) ([]byte, error)
		Set(rv map[string]string, r *http.Request, value []byte) error
	}
	KeyParams struct {
		Route []string
		Query []string
		Body  bool
	}
)

func (keyParams *KeyParams) GetKey(rv map[string]string, r *http.Request) (string, error) {
	buffer := bytes.NewBufferString("")
	for _, key := range keyParams.Route {
		value, ok := rv[key]
		if !ok {
			return "", fmt.Errorf("key not found")
		}
		buffer.WriteString(key)
		buffer.WriteString(":")
		buffer.WriteString(value)
		buffer.WriteString("\r\n")
	}

	for _, key := range keyParams.Query {
		value := r.URL.Query().Get(key)
		if len(value) == 0 {
			return "", fmt.Errorf("key not found")
		}
		buffer.WriteString(key)
		buffer.WriteString(":")
		buffer.WriteString(value)
		buffer.WriteString("\r\n")
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
