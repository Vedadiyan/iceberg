package filters

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/internal/logger"
	"github.com/vedadiyan/natsch"
)

type (
	NATSCHFilter struct {
		FilterBase
		Url      string
		Subject  string
		Deadline int
	}
)

func (filter *NATSCHFilter) Handle(r *http.Request) (*http.Response, error) {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	natschMsg := natsch.WrapMessage(msg)
	natschMsg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	msg.Reply = fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject)
	err = conn.PublishMsgSch(natschMsg)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	return nil, nil
}

func (filter *NATSCHFilter) HandleSync(r *http.Request) (*http.Response, error) {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	natschMsg := natsch.WrapMessage(msg)
	natschMsg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	msg.Reply = fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject)
	msg.Header.Add("reply", conn.NewRespInbox())
	var wg sync.WaitGroup
	var res *nats.Msg
	if len(filter.Filters) > 0 {
		wg.Add(1)
		unsubscriber, err := conn.Subscribe(msg.Reply, func(msg *nats.Msg) {
			res = msg
		})
		if err != nil {
			logger.Error(err, "")
			return nil, err
		}
		unsubscriber.AutoUnsubscribe(1)
	}
	err = conn.PublishMsgSch(natschMsg)
	if err != nil {
		logger.Error(err, "")
		return nil, err
	}
	wg.Wait()
	return MsgToResponse(res)
}

func (filter *NATSCHFilter) HandleAsync(r *http.Request) {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		logger.Error(err, "")
		return
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		logger.Error(err, "")
		return
	}
	natschMsg := natsch.WrapMessage(msg)
	natschMsg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	msg.Reply = fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject)
	err = conn.PublishMsgSch(natschMsg)
	if err != nil {
		logger.Error(err, "")
		return
	}
}

func (filter *NATSCHFilter) AddDurableSubscription() {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		logger.Error(err, "")
		return
	}
	_, err = conn.QueueSubscribeSch(fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject), "balanced", func(msg *natsch.Msg) {
		msg.Subject = msg.Reply
		msg.Reply = ""
		err := conn.Conn.PublishMsg(msg.Msg)
		if err != nil {
			logger.Error(err, "")
			return
		}
		req, err := RequestFrom(MsgToResponse(msg.Msg))
		if err != nil {
			logger.Error(err, "")
			return
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			logger.Error(err, "")
			return
		}
	})
	if err != nil {
		if err != nil {
			logger.Error(err, "")
			return
		}
	}
}
