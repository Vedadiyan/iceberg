package filters

import (
	"fmt"
	"log"
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
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	natschMsg := natsch.WrapMessage(msg)
	natschMsg.Deadline = time.Now().Add(time.Second * time.Duration(filter.Deadline)).UnixMicro()
	msg.Reply = fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject)
	err = conn.PublishMsgSch(natschMsg)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return nil, nil
}

func (filter *NATSCHFilter) HandleSync(r *http.Request) (*http.Response, error) {
	return filter.Handle(r)
}

func (filter *NATSCHFilter) HandleAsync(r *http.Request) {
	_, err := filter.Handle(r)
	if err != nil {
		log.Println(err)
	}
}

func (filter *NATSCHFilter) InitAsyncHandler() error {
	conn, err := di.ResolveWithName[natsch.Conn](filter.Url, nil)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = conn.Subscribe(fmt.Sprintf("ICEBERGREPLY.%s", filter.Subject), func(msg *nats.Msg) {
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
		return err
	}
	return nil
}
