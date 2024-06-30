package netio

import (
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
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
	(*u2).Host = u.Host
	(*u2).Scheme = u.Scheme
	return u2
}
func cloneURLValues(v url.Values) url.Values {
	if v == nil {
		return nil
	}
	return url.Values(http.Header(v).Clone())
}

func cloneTransferEncoding(transferEncoding []string) []string {
	if transferEncoding != nil {
		s2 := make([]string, len(transferEncoding))
		copy(s2, transferEncoding)
		return s2
	}
	return nil
}

func cloneHeader(header http.Header) http.Header {
	if header != nil {
		return header.Clone()
	}
	return nil
}

func cloneMultipartForm(f *multipart.Form) *multipart.Form {
	if f == nil {
		return nil
	}
	f2 := &multipart.Form{
		Value: (map[string][]string)(http.Header(f.Value).Clone()),
	}
	if f.File != nil {
		m := make(map[string][]*multipart.FileHeader)
		for k, vv := range f.File {
			vv2 := make([]*multipart.FileHeader, len(vv))
			for i, v := range vv {
				vv2[i] = cloneMultipartFileHeader(v)
			}
			m[k] = vv2
		}
		f2.File = m
	}
	return f2
}

func cloneMultipartFileHeader(fh *multipart.FileHeader) *multipart.FileHeader {
	if fh == nil {
		return nil
	}
	fh2 := new(multipart.FileHeader)
	*fh2 = *fh
	fh2.Header = textproto.MIMEHeader(http.Header(fh.Header).Clone())
	return fh2
}
