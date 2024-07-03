package netio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

type (
	RouteValues   map[string]string
	ShadowRequest struct {
		*http.Request
		RouteValue RouteValues
		data       []byte
	}
	RequestOption  func(*http.Request)
	RequestUpdater func(*ShadowRequest, *http.Request) error
)

func WithUrl(url *url.URL) RequestOption {
	return func(r *http.Request) {
		(*r).URL.Host = url.Host
		(*r).URL.Scheme = url.Scheme
		(*r).Host = url.Host
	}
}

func WithContext(ctx context.Context) RequestOption {
	return func(r *http.Request) {
		*r = *r.WithContext(ctx)
	}
}

func WithMethod(method string) RequestOption {
	return func(r *http.Request) {
		r.Method = method
	}
}

func ReqUpdateHeader(keys ...string) RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		for _, key := range keys {
			shadowRequest.Header.Set(key, r.Header.Get(key))
		}
		return nil
	}
}

func ReqUpdateTailer(keys ...string) RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		for _, key := range keys {
			shadowRequest.Trailer.Set(key, r.Header.Get(key))
		}
		return nil
	}
}

func ReqReplaceBody() RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		shadowRequest.data = body
		(*shadowRequest.Request).Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
}

func ReqReplaceHeader() RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		shadowRequest.Header = cloneHeader(r.Header)
		return nil
	}
}

func ReqReplaceTailer(keys ...string) RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		shadowRequest.Trailer = cloneHeader(r.Trailer)
		return nil
	}
}

func ReqReplaceForm() RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		shadowRequest.Form = cloneURLValues(r.Form)
		return nil
	}
}

func ReqReplaceURL() RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		shadowRequest.URL = cloneURL(r.URL)
		return nil
	}
}

func ReqReplaceMultipartForm() RequestUpdater {
	return func(shadowRequest *ShadowRequest, r *http.Request) error {
		shadowRequest.MultipartForm = cloneMultipartForm(r.MultipartForm)
		return nil
	}
}

func NewShadowRequest(request *http.Request) (*ShadowRequest, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	r := ShadowRequest{}
	r.Request = request
	(*r.Request).Body = io.NopCloser(bytes.NewReader(body))
	r.data = body
	return &r, nil
}

func UpdateRequest(shadowRequest *ShadowRequest, request *http.Request, requestUpdater []RequestUpdater) error {
	if requestUpdater == nil {
		return nil
	}
	for _, fn := range requestUpdater {
		err := fn(shadowRequest, request)
		if err != nil {
			return err
		}
	}
	return nil
}

func (shadowRequest *ShadowRequest) Reset() {
	(*shadowRequest.Request).Body = io.NopCloser(bytes.NewReader(shadowRequest.data))
}

func (shadowRequest *ShadowRequest) CloneRequest(options ...RequestOption) (*http.Request, error) {
	r := shadowRequest.Request
	req := new(http.Request)
	req.Header = cloneHeader(r.Header)
	req.Trailer = cloneHeader(r.Trailer)
	req.Form = cloneURLValues(r.Form)
	req.PostForm = cloneURLValues(r.PostForm)
	req.TransferEncoding = cloneTransferEncoding(r.TransferEncoding)
	req.Body = io.NopCloser(bytes.NewReader(shadowRequest.data))
	req.URL = cloneURL(r.URL)
	req.MultipartForm = cloneMultipartForm(r.MultipartForm)
	for _, option := range options {
		option(req)
	}
	return req, nil
}

func (shadowRequest *ShadowRequest) CloneShadowRequest(options ...RequestOption) (*ShadowRequest, error) {
	r := shadowRequest.Request
	req := new(http.Request)
	req.Header = cloneHeader(r.Header)
	req.Trailer = cloneHeader(r.Trailer)
	req.Form = cloneURLValues(r.Form)
	req.PostForm = cloneURLValues(r.PostForm)
	req.TransferEncoding = cloneTransferEncoding(r.TransferEncoding)
	req.Body = io.NopCloser(bytes.NewReader(shadowRequest.data))
	req.URL = cloneURL(r.URL)
	req.MultipartForm = cloneMultipartForm(r.MultipartForm)
	for _, option := range options {
		option(req)
	}
	return NewShadowRequest(req)
}
