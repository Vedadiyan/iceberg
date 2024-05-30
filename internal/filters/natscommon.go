package filters

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func GetMsg(r *http.Request, subject string) (*nats.Msg, error) {
	req, err := CloneRequest(r)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	msg := nats.Msg{}
	msg.Subject = subject
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

func GetStateManager(conn *nats.Conn) (nats.KeyValue, error) {
	js, err := conn.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: "ICEBERGSTATEMANAGEMENT",
		TTL:    time.Minute * 5,
	})
	if errors.Is(err, jetstream.ErrBucketExists) {
		return js.KeyValue("ICEBERGSTATEMANAGEMENT")
	}
	if err != nil {
		return nil, err
	}
	return kv, nil
}
func Await(conn *nats.Conn, awaitList []string) error {
	if len(awaitList) > 0 {
		var wg sync.WaitGroup
		stateManager, err := GetStateManager(conn)
		if err != nil {
			return err
		}
		for _, await := range awaitList {
			wg.Add(1)
			watcher, err := stateManager.Watch(await)
			if err != nil {
				return err
			}
			go func() {
				for update := range watcher.Updates() {
					if string(update.Value()) == "true" {
						wg.Done()
						return
					}
				}
			}()
		}
		wg.Wait()
	}
	return nil
}
