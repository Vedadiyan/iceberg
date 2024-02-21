package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/vedadiyan/goal/pkg/di"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type (
	GRPCFilter struct {
		FilterBase
		Url     string
		Subject string
	}
	GRPCCodec struct{}
)

var (
	_codec = GRPCCodec{}
)

// Marshal implements encoding.Codec.
func (GRPCCodec) Marshal(v any) ([]byte, error) {
	panic("unimplemented")
}

// Name implements encoding.Codec.
func (GRPCCodec) Name() string {
	panic("unimplemented")
}

// Unmarshal implements encoding.Codec.
func (GRPCCodec) Unmarshal(data []byte, v any) error {
	panic("unimplemented")
}

func (filter *GRPCFilter) Handle(r *http.Request) (*http.Response, error) {
	req, err := CloneRequest(r, WithUrl(filter.Address))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	client, err := di.ResolveWithName[grpc.ClientConn](filter.Url, nil)
	if err != nil {
		return nil, err
	}
	reply := make([]byte, 0)
	md := metadata.MD{}
	for key, values := range req.Header {
		md.Set(key, values...)
	}
	ctx := context.TODO()
	err = client.Invoke(ctx, filter.Subject, data, &reply, grpc.ForceCodec(_codec), grpc.Header(&md))
	if err != nil {
		return nil, err
	}
	response := http.Response{}
	response.Header = http.Header{}
	response.Body = io.NopCloser(bytes.NewBuffer(reply))
	headers, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for key, values := range headers {
			for _, value := range values {
				response.Header.Add(key, value)
			}
		}
	}
	return &response, nil
}
