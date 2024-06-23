package listeners

import (
	"net/http"

	"github.com/vedadiyan/iceberg/internal/conf"
)

func HandleCORS(conf *conf.Conf, w http.ResponseWriter, r *http.Request) bool {
	if conf.CORS != nil {
		if r.Method == "OPTIONS" {
			w.Header().Add("access-control-allow-origin", conf.CORS.Origins)
			w.Header().Add("access-control-allow-headers", conf.CORS.Headers)
			w.Header().Add("access-control-max-age", conf.CORS.MaxAge)
			w.Header().Add("access-control-allow-methods", conf.CORS.Methods)
			w.WriteHeader(200)
			return true
		}
		w.Header().Add("Access-Control-Expose-Headers", conf.CORS.ExposeHeader)
	}
	return false
}
