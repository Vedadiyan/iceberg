package filters

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	Level  int
	Filter struct {
		Name      string
		Address   *url.URL
		Level     Level
		Parallel  bool
		Timeout   time.Duration
		Callers   []netio.Caller
		AwaitList []string

		RequestUpdaters  []netio.RequestUpdater
		ResponseUpdaters []netio.ResponseUpdater

		instance netio.Caller
	}
)

const (
	LEVEL_CONNECT  Level = 1
	LEVEL_REQUEST  Level = 2
	LEVEL_RESPONSE Level = 4
)

func NewFilter() *Filter {
	f := new(Filter)
	f.RequestUpdaters = make([]netio.RequestUpdater, 0)
	f.ResponseUpdaters = make([]netio.ResponseUpdater, 0)
	return f
}

func (f *Filter) GetRequestUpdaters() []netio.RequestUpdater {
	return f.RequestUpdaters
}

func (f *Filter) GetResponseUpdaters() []netio.ResponseUpdater {
	return f.ResponseUpdaters
}

func (f *Filter) GetName() string {
	return f.Name
}

func (f *Filter) GetAwaitList() []string {
	return f.AwaitList
}

func (f *Filter) GetIsParallel() bool {
	return f.Parallel
}

func (f *Filter) GetContext() context.Context {
	ctx, cancel := context.WithCancel(context.TODO())
	time.AfterFunc(time.Until(time.Now().Add(f.Timeout)), func() {
		cancel()
	})
	return ctx
}

func (f *Filter) SetExchangeHeaders(headers []string) {
	switch f.Level {
	case LEVEL_CONNECT, LEVEL_REQUEST:
		{
			if len(headers) == 1 && headers[0] == "*" {
				f.RequestUpdaters = append(f.RequestUpdaters, netio.ReqReplaceHeader())
				return
			}
			f.RequestUpdaters = append(f.RequestUpdaters, netio.ReqUpdateHeader(headers...))
		}
	case LEVEL_RESPONSE:
		{
			if len(headers) == 1 && headers[0] == "*" {
				f.ResponseUpdaters = append(f.ResponseUpdaters, netio.ResReplaceHeader())
				return
			}
			f.ResponseUpdaters = append(f.ResponseUpdaters, netio.ResUpdateHeader(headers...))
		}
	}
}

func (f *Filter) SetExchangeBody(headers []string) {
	switch f.Level {
	case LEVEL_CONNECT, LEVEL_REQUEST:
		{
			f.RequestUpdaters = append(f.RequestUpdaters, netio.ReqReplaceBody())
		}
	case LEVEL_RESPONSE:
		{
			f.ResponseUpdaters = append(f.ResponseUpdaters, netio.ResReplaceBody())
		}
	}
}

func (f *Filter) Call(ctx context.Context, c netio.Cloner) (netio.Next, *http.Response, error) {
	return f.instance.Call(ctx, c)
}

func Await(resCh <-chan *netio.ShadowResponse, errCh <-chan error, ctx context.Context) (netio.Next, *http.Response, error) {
	select {
	case res := <-resCh:
		{
			return netio.CONTINUE, res.Response, nil
		}
	case err := <-errCh:
		{
			return netio.TERM, nil, err
		}
	case <-ctx.Done():
		{
			return netio.TERM, nil, context.DeadlineExceeded
		}
	}
}
