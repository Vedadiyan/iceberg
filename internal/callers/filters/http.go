package filters

import (
	"context"
	"net/http"

	"github.com/vedadiyan/iceberg/internal/common/netio"
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

func (f *HttpFilter) Call(ctx context.Context, rv netio.RouteValues, c netio.Cloner, _ netio.Cloner) (nxt netio.Next, rs *http.Response, e netio.Error) {

	rq, err := c(netio.WithUrl(f.Address, rv), netio.WithContext(ctx))
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}

	rs, err = http.DefaultClient.Do(rq)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusBadGateway)
	}
	if rs.StatusCode > 399 {
		return netio.TERM, nil, netio.NewError(rs.Status, rs.StatusCode)
	}
	return netio.CONTINUE, rs, nil
}
