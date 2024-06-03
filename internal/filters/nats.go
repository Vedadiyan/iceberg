package filters

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/internal/logger"
	natsqueue "github.com/vedadiyan/nats-helpers/queue"
)

type (
	NATSFilter struct {
		FilterBase
		Url                     string
		Subject                 string
		Queue                   *natsqueue.Queue
		queueInitializer        sync.Once
		stateManager            nats.KeyValue
		stateManagerInitializer sync.Once
		conn                    *nats.Conn
		connInitializer         sync.Once
		Durable                 bool
	}
)

func (filter *NATSFilter) GetQueue() *natsqueue.Queue {
	filter.queueInitializer.Do(func() {
		conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
		if err != nil {
			panic(err)
		}
		queue, err := natsqueue.New(conn, []string{filter.Subject}, filter.Name)
		if err != nil {
			panic(err)
		}
		filter.Queue = queue
	})
	return filter.Queue
}

func (filter *NATSFilter) GetStateManager() nats.KeyValue {
	filter.stateManagerInitializer.Do(func() {
		conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
		if err != nil {
			panic(err)
		}
		kv, err := GetStateManager(conn)
		if err != nil {
			panic(err)
		}
		filter.stateManager = kv
	})
	return filter.stateManager
}

func (filter *NATSFilter) GetConn() *nats.Conn {
	filter.connInitializer.Do(func() {
		conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
		if err != nil {
			panic(err)
		}
		filter.conn = conn
	})
	return filter.conn
}

func (filter *NATSFilter) BeginHandler(id string) error {
	_, err := filter.GetStateManager().Put(Key(filter.Name, id), []byte("false"))
	return err
}

func (filter *NATSFilter) EndHandler(id string) error {
	_, err := filter.GetStateManager().Put(Key(filter.Name, id), []byte("true"))
	return err
}

func (filter *NATSFilter) Await(id string) error {
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		return err
	}
	return Await(conn, filter.AwaitList, id)
}

func (filter *NATSFilter) BaseHandler(r *http.Request, handler func(*nats.Msg) error) (*http.Response, error) {

	id := r.Header.Get("x-request-id")
	if len(id) == 0 {
		return nil, fmt.Errorf("request id not found")
	}

	var wg sync.WaitGroup
	var res *nats.Msg

	queue := filter.GetQueue()
	err := filter.BeginHandler(id)
	if err != nil {
		return nil, err
	}
	err = filter.Await(id)
	if err != nil {
		return nil, err
	}

	msg, err := GetMsg(r, filter.Subject)
	if err != nil {
		return nil, err
	}

	msg.Reply = queue.Conn().NewRespInbox()

	wg.Add(1)

	queue.Conn().Subscribe(msg.Reply, func(msg *nats.Msg) {
		res = msg
		err := filter.EndHandler(id)
		if err != nil {
			logger.Error(err, "")
		}
		wg.Done()
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

	err = handler(msg)
	if err != nil {
		return nil, err
	}

	wg.Wait()

	return MsgToResponse(res)
}

func (filter *NATSFilter) HandleQueueSync(r *http.Request) (*http.Response, error) {
	return filter.BaseHandler(r, func(m *nats.Msg) error {
		return filter.GetQueue().PushMsg(m)
	})
}

func (filter *NATSFilter) HandleQueueAsync(r *http.Request) {
	go func() {
		_, err := filter.HandleQueueSync(r)
		if err != nil {
			logger.Error(err, "")
		}
	}()
}

func (filter *NATSFilter) HandleSimpleSync(r *http.Request) (*http.Response, error) {
	return filter.BaseHandler(r, func(m *nats.Msg) error {
		return filter.GetConn().PublishMsg(m)
	})
}

func (filter *NATSFilter) HandleSimpleAsync(r *http.Request) {
	go func() {
		_, err := filter.HandleSync(r)
		if err != nil {
			logger.Error(err, "")
		}
	}()
}

func (filter *NATSFilter) HandleSync(r *http.Request) (*http.Response, error) {
	if filter.Durable {
		return filter.HandleQueueSync(r)
	}
	return filter.HandleSimpleSync(r)
}

func (filter *NATSFilter) HandleAsync(r *http.Request) {
	if filter.Durable {
		filter.HandleQueueAsync(r)
		return
	}
	filter.HandleSimpleAsync(r)
}
