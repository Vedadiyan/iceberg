package handlers

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
)

type (
	NATSFilter struct {
		FilterBase
	}
)

func (filter *NATSFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Address.Scheme, nil)
	if err != nil {
		return nil, err
	}
	msg := nats.Msg{}
	msg.Subject = filter.Address.Opaque
	msg.Data = data
	msg.Header = nats.Header{}
	for key, values := range req.Header {
		for _, value := range values {
			msg.Header.Add(key, value)
		}
	}
	res, err := conn.RequestMsg(&msg, time.Minute)
	if err != nil {
		return nil, err
	}
	response := http.Response{}
	response.Header = http.Header{}
	response.Body = io.NopCloser(bytes.NewBuffer(res.Data))
	for key, values := range res.Header {
		for _, value := range values {
			response.Header.Add(key, value)
		}
	}
	return &response, nil
}
