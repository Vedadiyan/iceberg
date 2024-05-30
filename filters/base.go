package filters

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/iceberg/common"
)

type (
	Level  int
	Filter interface {
		Handle(r *http.Request) (*http.Response, error)
		HandleParellel(r *http.Request)
		Is(level Level) bool
		MoveTo(*http.Response, *http.Request) error
	}
	FilterBase struct {
		Filter
		Address         *url.URL
		ExchangeHeaders []string
		DropHeaders     []string
		ExchangeBody    bool
		Level           Level
		Timeout         int
		Filters         []Filter
	}
	Conf struct {
		Frontend *url.URL
		Backend  *url.URL
		Filters  []Filter
		CORS     *CORS
	}
	CORS struct {
		Origins      string
		Headers      string
		Methods      string
		ExposeHeader string
		MaxAge       string
	}
	Request struct {
		Url    *url.URL
		Method string
	}
	RequestOption func(*http.Request)
	KnownHeader   string
)

const (
	INHERIT  Level = 0
	REQUEST  Level = 2
	RESPONSE Level = 4
	CONNECT  Level = 8
	PARALLEL Level = 16

	HEADER_CONTINUE_ON_ERROR KnownHeader = "x-continue-on-error"
)

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}
	return u2
}
func cloneURLValues(v url.Values) url.Values {
	if v == nil {
		return nil
	}
	return url.Values(http.Header(v).Clone())
}

func CloneRequest(r *http.Request, options ...RequestOption) (*http.Request, error) {
	r2 := new(http.Request)
	request := Request{
		Url:    r.URL,
		Method: r.Method,
	}
	r2.URL = cloneURL(r.URL)
	if r.Header != nil {
		r2.Header = r.Header.Clone()
	}
	if r.Trailer != nil {
		r2.Trailer = r.Trailer.Clone()
	}
	if s := r.TransferEncoding; s != nil {
		s2 := make([]string, len(s))
		copy(s2, s)
		r2.TransferEncoding = s2
	}
	r2.Form = cloneURLValues(r.Form)
	r2.PostForm = cloneURLValues(r.PostForm)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	(*r).Body = io.NopCloser(bytes.NewBuffer(body))
	(*r2).Body = io.NopCloser(bytes.NewBuffer(body))
	(*r2).URL.Host = request.Url.Host
	(*r2).URL.Scheme = request.Url.Scheme
	(*r2).Host = request.Url.Host
	for _, option := range options {
		option(r2)
	}
	return r2, nil
}

func RequestFrom(res *http.Response, err error) (*http.Request, error) {
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequest("", "", res.Body)
	if err != nil {
		return nil, err
	}
	for key, values := range res.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	return r, nil
}

func MsgToResponse(msg *nats.Msg) (*http.Response, error) {
	response := http.Response{}
	response.Header = http.Header{}
	response.Body = io.NopCloser(bytes.NewBuffer(msg.Data))
	for key, values := range msg.Header {
		for _, value := range values {
			response.Header.Add(key, value)
		}
	}
	return &response, nil
}

func WithUrl(url *url.URL) RequestOption {
	return func(r *http.Request) {
		(*r).URL.Host = url.Host
		(*r).URL.Scheme = url.Scheme
		(*r).Host = url.Host
	}
}

func WithMethod(method string) RequestOption {
	return func(r *http.Request) {
		r.Method = method
	}
}

func (filter *FilterBase) Is(level Level) bool {
	return filter.Level&level == level
}

func (filter *FilterBase) MoveTo(res *http.Response, req *http.Request) error {
	if filter.Is(PARALLEL) {
		return nil
	}
	for _, header := range filter.ExchangeHeaders {
		values := res.Header.Get(header)
		req.Header.Del(header)
		req.Header.Add(header, values)
	}
	if filter.ExchangeBody {
		req.Body = res.Body
	}
	return nil
}

func HandleFilter(r *http.Request, filters []Filter, level Level) error {
	for _, filter := range filters {
		if !filter.Is(level) {
			continue
		}
		if !filter.Is(PARALLEL) {
			err := HandlerFunc(filter, r)
			if err != nil {
				return err
			}
			return nil
		}
		filter.HandleParellel(r)
	}
	return nil
}

func HandlerFunc(filter Filter, r *http.Request) error {
	res, err := filter.Handle(r)
	if err != nil {
		return common.NewHandlerError(common.HANDLER_ERROR_INTERNAL, 500, err.Error())
	}
	if res == nil {
		return nil
	}
	if res.Header.Get("status") != "200" && strings.ToLower(res.Header.Get(string(HEADER_CONTINUE_ON_ERROR))) != "true" {
		return common.NewHandlerError(common.HANDLER_ERROR_FILTER, res.StatusCode, res.Status)
	}
	err = filter.MoveTo(res, r)
	if err != nil {
		return err
	}
	return nil
}
