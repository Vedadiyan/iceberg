package bootstrap

import (
	"net/http"
	"net/url"

	"github.com/vedadiyan/iceberg/internal/common/router"
)

type (
	RouteValues         = router.RouteValues
	RegistrationOptions func(*Options, *router.RouteTable, *url.URL, func(w http.ResponseWriter, r *http.Request, rv RouteValues))
	Options             struct {
		Cors          bool
		ExposeHeaders string
	}
	CORS struct {
		AllowedOrigins string
		AllowedHeaders string
		AllowedMethods string
		MaxAge         string
		ExposedHeaders string
	}
)
