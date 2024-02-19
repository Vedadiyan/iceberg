package handlers

import (
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
)

type (
	NATSFilter struct {
		FilterBase
	}
)

func (filter *NATSFilter) Handle(r *http.Request) (*http.Response, error) {
	conn, err := di.ResolveWithName[nats.Conn](filter.Address.Scheme, nil)
	if err != nil {
		return nil, err
	}
	res, err := conn.Request(filter.Address.Opaque, nil, time.Minute)
	if err != nil {
		return nil, err
	}
	_ = res
	return nil, nil
}
