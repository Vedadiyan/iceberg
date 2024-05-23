package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vedadiyan/goal/pkg/di"
)

type (
	NATSCHFilter struct {
		FilterBase
		Url      string
		Subject  string
		Deadline time.Time
		Headers  map[string]any
	}
)

func (filter *NATSCHFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	conn, err := di.ResolveWithName[nats.Conn](filter.Url, nil)
	if err != nil {
		return nil, err
	}
	msg := nats.Msg{}
	msg.Subject = filter.Subject
	msg.Data = data
	msg.Header = nats.Header{}
	for key, values := range req.Header {
		for _, value := range values {
			msg.Header.Add(key, value)
		}
	}
	msg.Header.Add("path", req.URL.Path)
	msg.Header.Add("query", req.URL.RawQuery)
	for key, value := range filter.Headers {
		switch value := value.(type) {
		case string, int64, float64, bool:
			{
				msg.Header.Add(key, fmt.Sprintf("%v", value))
			}
		case []any:
			{
				var buffer bytes.Buffer
				for _, value := range value {
					buffer.WriteString(fmt.Sprintf("%v", value))
					buffer.WriteString(",")
				}
				msg.Header.Add(key, string(buffer.Bytes()[:buffer.Len()-1]))
			}
		default:
			{
				return nil, fmt.Errorf("invalid header type %T", value)
			}
		}
	}
	res, err := conn.RequestMsg(&msg, time.Second*time.Duration(filter.Timeout))
	if err != nil {
		return nil, err
	}
	response := http.Response{}
	response.Header = http.Header{}
	response.Body = io.NopCloser(bytes.NewBuffer(res.Data))
	for key, values := range res.Header {
		for _, value := range values {
			response.Header.Add(key, value)
		}
	}
	return &response, nil
}
