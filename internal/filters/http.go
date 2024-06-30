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
	return httpFilter
}

func (f *HttpFilter) Call(ctx context.Context, c netio.Cloner) (*http.Response, error) {
	r, err := c(netio.WithUrl(f.Address), netio.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(r)
}
