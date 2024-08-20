package common

import (
	"context"
	"net/http"

	"github.com/vedadiyan/iceberg/internal/common/netio"
)

type (
	Logger interface {
		Log(string, *LogContext)
	}
	LogContext struct {
		CurrentHttp netio.Cloner
		OldHttp     netio.Cloner
		RouteValues netio.RouteValues
		Scope       string
		Error       error
		Params      map[string]string
	}
	Loggable struct {
		instance netio.Caller
		logger   Logger
	}
)

func NewLoggable(c netio.Caller, l Logger) *Loggable {
	lg := new(Loggable)
	lg.instance = c
	lg.logger = l
	return lg
}

func (l *Loggable) Call(ctx context.Context, rv netio.RouteValues, c netio.Cloner, o netio.Cloner) (netio.Next, *http.Response, netio.Error) {
	logCtx := new(LogContext)
	logCtx.CurrentHttp = c
	logCtx.OldHttp = o
	logCtx.RouteValues = rv
	logCtx.Scope = l.instance.GetName()

	l.logger.Log("INFO", logCtx)
	n, r, err := l.instance.Call(ctx, rv, c, o)
	defer func() {
		if err != nil {
			logCtx.Error = err
			l.logger.Log("ERROR", logCtx)
			return
		}
		l.logger.Log("INFO", logCtx)
	}()
	return n, r, err
}
func (l *Loggable) GetLevel() netio.Level {
	return l.instance.GetLevel()
}
func (l *Loggable) GetIsParallel() bool {
	return l.instance.GetIsParallel()
}
func (l *Loggable) GetName() string {
	return l.instance.GetName()
}
func (l *Loggable) GetAwaitList() []string {
	return l.instance.GetAwaitList()
}
func (l *Loggable) GetRequestUpdaters() []netio.RequestUpdater {
	return l.instance.GetRequestUpdaters()
}
func (l *Loggable) GetResponseUpdaters() []netio.ResponseUpdater {
	return l.instance.GetResponseUpdaters()
}
func (l *Loggable) OverrideRequestUpdaters(r []netio.RequestUpdater) {
	l.instance.OverrideRequestUpdaters(r)
}
func (l *Loggable) OverrideResponseUpdaters(r []netio.ResponseUpdater) {
	l.instance.OverrideResponseUpdaters(r)
}
func (l *Loggable) GetContext() context.Context {
	return l.instance.GetContext()
}
