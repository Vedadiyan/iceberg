package filters

import (
	"bytes"
	"io"
	"net/http"

	"github.com/nats-io/nats.go"
)

func (filter *NATSFilter) GetMsg(r *http.Request) (*nats.Msg, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
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
	return &msg, nil
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
