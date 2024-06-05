package filters

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/internal/logger"
	natshelpers "github.com/vedadiyan/nats-helpers"
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
		ReflectionKey           string
	}
)

var (
	_reflectors map[string]bool
	_mutRw      sync.RWMutex
)

func init() {
	_reflectors = make(map[string]bool)
}

func (filter *NATSFilter) GetQueue() *natsqueue.Queue {
	filter.queueInitializer.Do(func() {
		conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
		if err != nil {
			panic(err)
		}
		queue, err := natsqueue.New(conn, []string{filter.Subject})
		if err != nil {
			panic(err)
		}
		filter.Queue = queue
		logger.Info("created", filter.Subject)
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

	reply := queue.Conn().NewRespInbox()
	msg.Header.Set("Reply", reply)
	msg.Header.Set("Reflector", filter.ReflectionKey)

	wg.Add(1)

	subs, err := queue.Conn().Subscribe(reply, func(msg *nats.Msg) {
		defer wg.Done()
		logger.Info("received", filter.Subject)
		res = msg
		err := filter.EndHandler(id)
		if err != nil {
			logger.Error(
				err,
				logger.NameOfFunc(filter.BaseHandler),
				logger.NameOfFunc(filter.EndHandler),
			)
		}
	})
	if err != nil {
		return nil, err
	}

	err = subs.AutoUnsubscribe(1)
	if err != nil {
		return nil, err
	}

	logger.Info("pushing", filter.Subject)
	err = handler(msg)
	if err != nil {
		return nil, err
	}
	logger.Info("pushed", filter.Subject)
	wg.Wait()

	logger.Info("done", filter.Subject)

	return MsgToResponse(res)
}

func (filter *NATSFilter) HandleDurableSync(r *http.Request) (*http.Response, error) {
	return filter.BaseHandler(r, func(m *nats.Msg) error {
		logger.Info("SENDING", filter.Subject)
		m.Header.Set("X-Status", m.Header.Get("Status"))
		m.Header.Del("Status")
		return filter.GetQueue().PushMsg(m)
	})
}

func (filter *NATSFilter) HandleDurableAsync(r *http.Request) {
	go func() {
		_, err := filter.HandleDurableSync(r)
		if err != nil {
			logger.Error(err, logger.NameOfFunc(filter.HandleDurableAsync))
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
			logger.Error(err, logger.NameOfFunc(filter.HandleSimpleAsync))
		}
	}()
}

func (filter *NATSFilter) HandleSync(r *http.Request) (*http.Response, error) {
	if filter.Durable {
		return filter.HandleDurableSync(r)
	}
	return filter.HandleSimpleSync(r)
}

func (filter *NATSFilter) HandleAsync(r *http.Request) {
	if filter.Durable {
		filter.HandleDurableAsync(r)
		return
	}
	filter.HandleSimpleAsync(r)
}

func (filter *NATSFilter) InitializeReflector() error {
	_mutRw.RLock()
	if _, ok := _reflectors[filter.ReflectionKey]; ok {
		_mutRw.RUnlock()
		return nil
	}
	_mutRw.RUnlock()
	_mutRw.Lock()
	defer _mutRw.Unlock()
	_reflectors[filter.ReflectionKey] = true
	if filter.Durable {
		return filter.DurableReflector()
	}
	return filter.SimpleReflector()
}

func (filter *NATSFilter) DurableReflector() error {
	conn := filter.GetConn()
	queue, err := natsqueue.New(conn, []string{filter.ReflectionKey})
	if err != nil {
		return err
	}
	_, err = queue.Pull(filter.ReflectionKey, func(m *nats.Msg) natshelpers.State {
		msg := *m
		msg.Subject = msg.Header.Get("Reply")
		msg.Reply = ""
		err := conn.PublishMsg(&msg)
		if err != nil {
			return natshelpers.Drop()
		}
		req, err := RequestFrom(MsgToResponse(&msg))
		if err != nil {
			return natshelpers.Drop()
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			return natshelpers.Drop()
		}
		return natshelpers.Done()
	})
	return err
}

func (filter *NATSFilter) SimpleReflector() error {
	conn := filter.GetConn()
	_, err := conn.Subscribe(filter.ReflectionKey, func(m *nats.Msg) {
		msg := *m
		msg.Subject = msg.Header.Get("Reply")
		err := conn.PublishMsg(&msg)
		if err != nil {
			logger.Error(
				err,
				logger.NameOfFunc(filter.SimpleReflector),
				logger.NameOfFunc(conn.PublishMsg),
			)
			return
		}
		req, err := RequestFrom(MsgToResponse(&msg))
		if err != nil {
			logger.Error(
				err,
				logger.NameOfFunc(filter.SimpleReflector),
				logger.NameOfFunc(RequestFrom),
			)
			return
		}
		err = HandleFilter(req, filter.Filters, INHERIT)
		if err != nil {
			logger.Error(
				err,
				logger.NameOfFunc(filter.SimpleReflector),
				logger.NameOfFunc(HandleFilter),
			)
			return
		}
	})
	return err
}
