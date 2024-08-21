package filters

import (
	"context"
	"net/http"

	"github.com/vedadiyan/iceberg/internal/common/logging"
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

func (f *HttpFilter) Call(ctx context.Context, rv netio.RouteValues, c netio.Cloner, _ netio.Cloner) (_n netio.Next, _r *http.Response, _e netio.Error) {
	log := logging.GetLogger(f.Logger)
	log.Init(f.Metadata())
	defer log.Close(_n == netio.TERM, _e)

	r, err := c(netio.WithUrl(f.Address, rv), netio.WithContext(ctx))
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusInternalServerError)
	}
	log.Trace(f.Tracedata("Call", r.Header, rv, r.URL, nil))
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return netio.TERM, nil, netio.NewError(err.Error(), http.StatusBadGateway)
	}
	if res.StatusCode > 399 {
		return netio.TERM, nil, netio.NewError(res.Status, res.StatusCode)
	}
	return netio.CONTINUE, res, nil
}
