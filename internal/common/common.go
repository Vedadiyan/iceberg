package common

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/caches"
	"github.com/vedadiyan/iceberg/internal/filters"
)

type (
	Handler func(*http.ServeMux)
	Conf    struct {
		Frontend *url.URL
		Backend  *url.URL
		Filters  []filters.Filter
		CORS     *CORS
		Cache    caches.Cache
	}

	CORS struct {
		Origins      string
		Headers      string
		Methods      string
		ExposeHeader string
		MaxAge       string
	}
)
