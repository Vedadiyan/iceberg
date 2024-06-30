package filters

import (
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

func (f *HttpFilter) Call(c netio.Cloner) (*http.Response, error) {
	r, err := c(netio.WithUrl(f.Address))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(r)
}
