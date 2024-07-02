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

func (f *HttpFilter) Call(ctx context.Context, c netio.Cloner) (bool, *http.Response, error) {
	r, err := c(netio.WithUrl(f.Address), netio.WithContext(ctx))
	if err != nil {
		return false, nil, err
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return false, nil, err
	}
	return false, res, nil
}
