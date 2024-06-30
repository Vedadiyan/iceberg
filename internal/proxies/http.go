package proxies

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/netio"
)

type (
	HttpProxy struct {
		url             *url.URL
		requestUpdater  []netio.RequestUpdater
		responseUpdater []netio.ResponseUpdater
	}
)

func NewHttpProxy(url *url.URL) *HttpProxy {
	httpProxy := new(HttpProxy)
	httpProxy.url = url
	httpProxy.responseUpdater = make([]netio.ResponseUpdater, 0)
	httpProxy.requestUpdater = make([]netio.RequestUpdater, 0)
	httpProxy.responseUpdater = append(httpProxy.responseUpdater, netio.ResReplaceBody(), netio.ResUpdateHeaders(), netio.ResUpdateTailers())
	return httpProxy
}

func (f *HttpProxy) Call(c netio.Cloner) (*http.Response, error) {
	r, err := c(netio.WithUrl(f.url))
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(r)
}

func (f *HttpProxy) RequestUpdaters() []netio.RequestUpdater {
	return f.requestUpdater
}

func (f *HttpProxy) ResponseUpdaters() []netio.ResponseUpdater {
	return f.responseUpdater
}

func (f *HttpProxy) IsParallel() bool {
	return false
}
