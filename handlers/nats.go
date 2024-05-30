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
		Url     string
		Subject string
	}
)

func (filter *NATSFilter) Prepare(r *http.Request) (*nats.Msg, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	msg := nats.Msg{}
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
	return &msg, nil
}

func (filter *NATSFilter) Handle(r *http.Request) (*http.Response, error) {
	msg, err := filter.Prepare(r)
	if err != nil {
		return nil, err
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		return nil, err
	}
	res, err := conn.RequestMsg(msg, time.Second*time.Duration(filter.Timeout))
	if err != nil {
		return nil, err
	}
	return MsgToResponse(res)
}

func MsgToResponse(msg *nats.Msg) (*http.Response, error) {
	response := http.Response{}
	response.Header = http.Header{}
	response.Body = io.NopCloser(bytes.NewBuffer(msg.Data))
	for key, values := range msg.Header {
		for _, value := range values {
			response.Header.Add(key, value)
		}
	}
	return &response, nil
}

func (filter *NATSFilter) HandleParellel(r *http.Request) error {
	msg, err := filter.Prepare(r)
	if err != nil {
		return err
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		return err
	}
	msg.Reply = conn.NewRespInbox()
	unsubscriber, err := conn.Subscribe(msg.Reply, func(msg *nats.Msg) {
		req, err := RequestFrom(MsgToResponse(msg))
		if err != nil {

		}
		_ = HandleFilter(req, filter.Filters, RESPONSE)
	})
	if err != nil {
		return err
	}
	unsubscriber.AutoUnsubscribe(1)
	err = conn.PublishMsg(msg)
	if err != nil {
		return err
	}
	return nil
}
