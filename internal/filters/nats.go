package filters

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/internal/logger"
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
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		logger.Error(err, "")
		return
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		logger.Error(err, "")
		return
	}
	stateManager, err := GetStateManager(conn)
	if err != nil {
		logger.Error(err, "")
		return
	}
	key := fmt.Sprintf("%s_%s", filter.Name, r.Header.Get("x-request-id"))
	_, err = stateManager.Put(key, []byte("false"))
	if err != nil {
		logger.Error(err, "")
		return
	}
	if len(filter.Filters) > 0 {
		msg.Reply = conn.NewRespInbox()
		unsubscriber, err := conn.Subscribe(msg.Reply, func(msg *nats.Msg) {
			_, err = stateManager.Put(key, []byte("true"))
			if err != nil {
				logger.Error(err, "")
				return
			}
			req, err := RequestFrom(MsgToResponse(msg))
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
			logger.Error(err, "")
			return
		}
		unsubscriber.AutoUnsubscribe(1)
	}
	err = conn.PublishMsg(msg)
	if err != nil {
		logger.Error(err, "")
		return
	}
}
