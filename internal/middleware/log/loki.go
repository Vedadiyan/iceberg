package log

import (
	"context"
	"net/http"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	LokiLog struct {
		*Log
		get *LokiBegin
		set *LokiEnd
	}
	LokiBegin struct {
		*LokiLog
	}
	LokiEnd struct {
		*LokiLog
	}
)

func NewLokiLog(l *Log) (*LokiLog, error) {
	panic("not implemented")
}

func (lokiLog *LokiLog) Get() netio.Caller {
	return lokiLog.get
}

func (lokiLog *LokiLog) Set() netio.Caller {
	return lokiLog.set
}

func (l *LokiBegin) GetLevel() netio.Level {
	return netio.LEVEL_BEGIN
}

func (l *LokiEnd) GetLevel() netio.Level {
	return netio.LEVEL_END
}

func (f *LokiBegin) Call(ctx context.Context, rv netio.RouteValues, in netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	return netio.CONTINUE, nil, nil
}

func (f *LokiEnd) Call(ctx context.Context, rv netio.RouteValues, in netio.Cloner, _ netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	return netio.CONTINUE, nil, nil
}
