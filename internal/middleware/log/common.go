package log

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	Log struct {
		Address   *url.URL
		Fallbacks []string
		Metadata  map[string]string
	}
)

func (l *Log) GetRequestUpdaters() []netio.RequestUpdater {
	return nil
}

func (l *Log) GetResponseUpdaters() []netio.ResponseUpdater {
	return nil
}

func (l *Log) OverrideRequestUpdaters([]netio.RequestUpdater) {

}

func (l *Log) OverrideResponseUpdaters([]netio.ResponseUpdater) {

}

func (l *Log) GetName() string {
	return "Log"
}

func (l *Log) GetAwaitList() []string {
	return nil
}

func (l *Log) GetIsParallel() bool {
	return false
}

func (l *Log) GetContext() context.Context {
	return context.TODO()
}

func (l *Log) Build() ([]netio.Caller, error) {
	switch strings.ToLower("") {
	case "loki":
		{
			loki, err := NewLokiLog(l)
			if err != nil {
				return nil, err
			}
			return []netio.Caller{loki.Get(), loki.Set()}, nil
		}
	}
	return nil, fmt.Errorf("unsupported scheme %s", "")
}
