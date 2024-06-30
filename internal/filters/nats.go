package filters

import (
	"context"
	"net/http"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/internal/di"
	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	NATSFilter struct {
		*Filter
		Durable bool
	}
)

func (natsFilter *NATSFilter) Call(ctx context.Context, c netio.Cloner) (*http.Response, error) {
	if !natsFilter.Durable {
		return natsFilter.Simple()
	}
	panic("")
}

func (natsFilter *NATSFilter) Simple() (*http.Response, error) {
	var (
		res *http.Response
		err error
	)
	conn, err := di.Resolve[*nats.Conn]()
	if err != nil {
		return nil, err
	}
	inbox := conn.NewRespInbox()
	var wg sync.WaitGroup
	subs, err := conn.Subscribe(inbox, func(msg *nats.Msg) {
		go func() {
			defer wg.Done()

		}()
	})
	if err != nil {
		return nil, err
	}
	err = subs.AutoUnsubscribe(1)
	if err != nil {
		return nil, err
	}
	err = conn.PublishMsg(&nats.Msg{})
	if err != nil {
		return nil, err
	}
	wg.Wait()
	return res, err
}
