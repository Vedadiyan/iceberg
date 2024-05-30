package common

import "net/http"

type (
	Handler func(*http.ServeMux)
)
