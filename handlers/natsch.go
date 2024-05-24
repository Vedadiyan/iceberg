package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/natsch"
)

type (
	NATSCHFilter struct {
		FilterBase
		Url       string
		Subject   string
		Deadline  int
		Callbacks map[string][]string
	}
)

func (filter *NATSCHFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		return nil, err
	}
	msg := natsch.Msg{}
	msg.Subject = filter.Subject
	msg.Data = data
	msg.Header = nats.Header{}
	for key, values := range req.Header {
		for _, value := range values {
			msg.Header.Add(key, value)
		}
	}
	msg.Header.Add("path", req.URL.Path)
	msg.Header.Add("query", req.URL.RawQuery)
	for key, values := range filter.Callbacks {
		msg.Header.Add("x-callbacks", key)
		key := fmt.Sprintf("x-callback-%s", key)
		for _, value := range values {
			msg.Header.Add(key, value)
		}
	}
	msg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	err = conn.PublishMsgSch(&msg)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
