package filters

import (
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

func (filter *NATSFilter) HandleSync(r *http.Request) (*http.Response, error) {
	msg, err := GetMsg(r, filter.Subject)
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
	if len(filter.Filters) > 0 {
		req, err := RequestFrom(MsgToResponse(msg))
		if err != nil {
			return nil, err
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			return nil, err
		}
	}
	return MsgToResponse(res)
}

func (filter *NATSFilter) HandleAsync(r *http.Request) {
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		log.Println(err)
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		log.Println(err)
	}
	if len(filter.Filters) > 0 {
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
	}
	err = conn.PublishMsg(msg)
	if err != nil {
		log.Println(err)
	}
}