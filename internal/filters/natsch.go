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
	Await(conn.Conn, filter.AwaitList)
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
			wg.Done()
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
	Await(conn.Conn, filter.AwaitList)
	stateManager, err := GetStateManager(conn.Conn)
	if err != nil {
		logger.Error(err, "")
		return
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		logger.Error(err, "")
		return
	}
	key := fmt.Sprintf("%s_%s", filter.Name, r.Header.Get("x-request-id"))
	natschMsg := natsch.WrapMessage(msg)
	natschMsg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	msg.Reply = fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject)
	msg.Header.Add("reply", conn.NewRespInbox())
	_, err = stateManager.Create(key, []byte("false"))
	if err != nil {
		logger.Error(err, "")
		return
	}
	if len(filter.Filters) > 0 {
		unsubscriber, err := conn.Subscribe(msg.Reply, func(msg *nats.Msg) {
			_, err = stateManager.Put(key, []byte("true"))
			if err != nil {
				logger.Error(err, "")
				return
			}
		})
		if err != nil {
			logger.Error(err, "")
		} else {
			unsubscriber.AutoUnsubscribe(1)
		}
	}
	err = conn.PublishMsgSch(natschMsg)
	if err != nil {
		logger.Error(err, "")
		return
	}
}

func (filter *NATSCHFilter) AddDurableSubscription() error {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		return err
	}
	_, err = conn.QueueSubscribeSch(fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject), "balanced", func(msg *natsch.Msg) {
		msg.Subject = msg.Reply
		msg.Reply = ""
		err := conn.Conn.PublishMsg(msg.Msg)
		if err != nil {
			logger.Error(err, "")
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
		return err
	}
	return nil
}
