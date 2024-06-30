package netio

import (
	"bytes"
	"io"
	"net/http"
)

type (
	ShadowResponse struct {
		*http.Response
		data []byte
	}
	ResponseUpdater func(*ShadowResponse, *http.Response) error
)

func NewShandowResponse(response *http.Response) (*ShadowResponse, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	r := ShadowResponse{}
	r.Response = response
	(*r.Response).Body = io.NopCloser(bytes.NewBuffer(body))
	r.data = body
	return &r, nil
}

func UpdateResponse(shadowResponse *ShadowResponse, response *http.Response, requestUpdater []ResponseUpdater) error {
	for _, fn := range requestUpdater {
		err := fn(shadowResponse, response)
		if err != nil {
			return err
		}
	}
	return nil
}

func ResUpdateHeaders() ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		for key, values := range r.Header {
			for _, value := range values {
				shadowResponse.Response.Header.Add(key, value)
			}
		}
		return nil
	}
}

func ResUpdateTailers() ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		for key, values := range r.Trailer {
			for _, value := range values {
				shadowResponse.Response.Trailer.Add(key, value)
			}
		}
		return nil
	}
}

func ResUpdateHeader(keys ...string) ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		for _, key := range keys {
			shadowResponse.Header.Set(key, r.Header.Get(key))
		}
		return nil
	}
}

func ResUpdateTailer(keys ...string) ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		for _, key := range keys {
			shadowResponse.Trailer.Set(key, r.Header.Get(key))
		}
		return nil
	}
}

func ResReplaceBody() ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		shadowResponse.data = body
		(*shadowResponse.Response).Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
}

func ResReplaceHeader() ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		shadowResponse.Header = cloneHeader(r.Header)
		return nil
	}
}

func ResReplaceTailer(keys ...string) ResponseUpdater {
	return func(shadowResponse *ShadowResponse, r *http.Response) error {
		shadowResponse.Trailer = cloneHeader(r.Trailer)
		return nil
	}
}

func (shadowResponse *ShadowResponse) Reset() {
	(*shadowResponse.Request).Body = io.NopCloser(bytes.NewReader(shadowResponse.data))
}

func (shadowResponse *ShadowResponse) CreateRequest() (*ShadowRequest, error) {
	req := http.Request{
		Header:           cloneHeader(shadowResponse.Header),
		Trailer:          cloneHeader(shadowResponse.Trailer),
		TransferEncoding: cloneTransferEncoding(shadowResponse.TransferEncoding),
		Body:             io.NopCloser(bytes.NewReader(shadowResponse.data)),
	}
	return NewShadowRequest(&req)
}

func (shadowResponse *ShadowResponse) CloneResponse() (*http.Response, error) {
	r := shadowResponse.Response
	res := *r
	res.Body = io.NopCloser(bytes.NewReader(shadowResponse.data))
	return &res, nil
}

func (shadowResponse *ShadowResponse) Write(w http.ResponseWriter) {
	for key, values := range shadowResponse.Response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Del("Content-Length")
	w.Write(shadowResponse.data)
}
