package proxies

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	HttpProxy struct {
		Name      string
		Address   *url.URL
		Timeout   time.Duration
		Callers   []netio.Caller
		AwaitList []string

		RequestUpdaters  []netio.RequestUpdater
		ResponseUpdaters []netio.ResponseUpdater
	}
)

func NewHttpProxy(address *url.URL, callers []netio.Caller) *HttpProxy {
	httpProxy := new(HttpProxy)
	httpProxy.Address = address
	httpProxy.Callers = netio.Sort(append(callers, httpProxy)...)
	httpProxy.ResponseUpdaters = make([]netio.ResponseUpdater, 0)
	httpProxy.RequestUpdaters = make([]netio.RequestUpdater, 0)
	httpProxy.ResponseUpdaters = append(httpProxy.ResponseUpdaters, netio.ResReplaceBody(), netio.ResUpdateHeaders(), netio.ResUpdateTailers())
	return httpProxy
}

func (f *HttpProxy) GetRequestUpdaters() []netio.RequestUpdater {
	return f.RequestUpdaters
}

func (f *HttpProxy) GetResponseUpdaters() []netio.ResponseUpdater {
	return f.ResponseUpdaters
}

func (f *HttpProxy) GetName() string {
	return f.Name
}

func (f *HttpProxy) GetAwaitList() []string {
	return f.AwaitList
}

func (f *HttpProxy) GetIsParallel() bool {
	return false
}

func (f *HttpProxy) GetContext() context.Context {
	ctx, cancel := context.WithCancel(context.TODO())
	timeout := f.Timeout
	if timeout == 0 {
		timeout = time.Second * 30
	}
	time.AfterFunc(time.Until(time.Now().Add(timeout)), func() {
		cancel()
	})
	return ctx
}

func (f *HttpProxy) GetLevel() netio.Level {
	return netio.LEVEL_NONE
}

func (f *HttpProxy) Call(ctx context.Context, _ netio.RouteValues, c netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
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

func (f *HttpProxy) Handle(w http.ResponseWriter, r *http.Request) {
	in, err := netio.NewShadowRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	out, _err := netio.Cascade(in, f.Callers...)
	if _err != nil {
		http.Error(w, _err.Message(), _err.Status())
	}
	out.Write(w)
}
