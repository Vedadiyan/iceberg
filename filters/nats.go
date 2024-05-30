package filters

import (
	"io"
	"log"
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

func (filter *NATSFilter) GetMsg(r *http.Request) (*nats.Msg, error) {
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

func (filter *NATSFilter) HandleSync(r *http.Request) (*http.Response, error) {
	msg, err := filter.GetMsg(r)
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

func (filter *NATSFilter) HandleAsync(r *http.Request) {
	msg, err := filter.GetMsg(r)
	if err != nil {
		log.Println(err)
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		log.Println(err)
	}
	msg.Reply = conn.NewRespInbox()
	unsubscriber, err := conn.Subscribe(msg.Reply, func(msg *nats.Msg) {
		req, err := RequestFrom(MsgToResponse(msg))
		if err != nil {
			log.Println(err)
			return
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			log.Println(err)
		}
	})
	if err != nil {
		log.Println(err)
	}
	unsubscriber.AutoUnsubscribe(1)
	err = conn.PublishMsg(msg)
	if err != nil {
		log.Println(err)
	}
}
