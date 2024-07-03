package filters

import (
	"context"
	"net/http"

	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	HttpFilter struct {
		*Filter
	}
)

func NewHttpFilter(f *Filter) *HttpFilter {
	httpFilter := new(HttpFilter)
	httpFilter.Filter = f
	f.instance = httpFilter
	return httpFilter
}

func (f *HttpFilter) Call(ctx context.Context, c netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	r, err := c(netio.WithUrl(f.Address), netio.WithContext(ctx))
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusBadGateway)
	}
	return netio.CONTINUE, res, nil
}
