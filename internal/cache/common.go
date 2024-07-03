package cache

import (
	"net/http"
	"time"
)

type (
	Cache struct {
		KeyTemplate string
		TTL         time.Duration
	}
)

func (c *Cache) ParseKey(r http.Request) (string, error) {
	panic("")
}
