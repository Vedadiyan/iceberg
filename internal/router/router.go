package router

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type (
	RouterError string
	RouteValues map[string]string
	HandlerFunc func(http.ResponseWriter, *http.Request, RouteValues)
	RouteTable  struct {
		routes  map[int][]*Route
		configs map[string]HandlerFunc
	}
	Route struct {
		host        string
		method      string
		routeValues map[int]string
		routeParams map[int]string
		hash        string
	}
)

const (
	NO_MATCH_FOUND    RouterError = "no match found"
	NO_URL_REGISTERED RouterError = "no url registered"
)

var (
	_routeTable RouteTable
	_once       sync.Once
)

func (routerError RouterError) Error() string {
	return string(routerError)
}

func (r *Route) Bind(route *Route) map[string]string {
	rank := RouteCompare(r, route)
	if rank == 0 {
		return nil
	}
	routeValues := make(map[string]string)
	for key, value := range r.routeValues {
		k := r.routeValues[key]
		if value == "?" {
			k = r.routeParams[key]
		}
		routeValues[k] = route.routeValues[key]
	}
	return routeValues
}

func (route *Route) GetHash() string {
	return route.hash
}

func ParseRoute(url *url.URL, method string) *Route {
	routeValues := make(map[int]string)
	routeParams := make(map[int]string)
	for index, segment := range strings.Split(url.Path, "/") {
		if len(segment) == 0 {
			continue
		}
		if strings.HasPrefix(segment, ":") {
			routeValues[index] = "?"
			routeParams[index] = segment[1:]
			continue
		}
		routeValues[index] = segment
		routeValues[index] = segment
	}

	hash := CreateHash(url, method)
	route := Route{
		host:        url.Host,
		routeValues: routeValues,
		routeParams: routeParams,
		method:      strings.ToUpper(method),
		hash:        hash,
	}
	return &route
}

func RouteCompare(preferredRoute *Route, route *Route) int {
	if len(preferredRoute.routeValues) != len(route.routeValues) {
		return 0
	}
	rank := 0
	for key, value := range preferredRoute.routeValues {
		if value == "?" {
			rank += 1
			continue
		}
		if value != route.routeValues[key] {
			rank = 0
			break
		}
		rank += 2
	}
	return rank
}

func CreateHash(url *url.URL, method string) string {
	buffer := bytes.NewBufferString(strings.ToUpper(method))
	buffer.WriteString(":")
	buffer.WriteString(url.Path)
	sha256 := sha256.New()
	sha256.Write(buffer.Bytes())
	hash := hex.EncodeToString(sha256.Sum(nil))
	return hash
}

func DefaultRouteTable() *RouteTable {
	_once.Do(func() {
		_routeTable = RouteTable{
			routes:  map[int][]*Route{},
			configs: make(map[string]HandlerFunc),
		}
	})
	return &_routeTable
}

func (rt *RouteTable) Register(url *url.URL, method string, handlerFunc HandlerFunc) {
	route := ParseRoute(url, method)
	len := len(route.routeValues)
	if _, ok := rt.configs[route.hash]; ok {
		return
	}
	rt.configs[route.hash] = handlerFunc
	_, ok := rt.routes[len]
	if !ok {
		rt.routes[len] = make([]*Route, 0)
	}
	rt.routes[len] = append(rt.routes[len], route)
}

func (rt RouteTable) Find(url *url.URL, method string) (http.HandlerFunc, error) {
	if len(rt.routes) == 0 {
		return nil, NO_URL_REGISTERED
	}
	prt := ParseRoute(url, method)
	routes, ok := rt.routes[len(prt.routeValues)]
	if !ok {
		return nil, NO_MATCH_FOUND
	}
	lrnk := 0
	var lrt *Route
	for _, url := range routes {
		if url.method != strings.ToUpper(method) {
			continue
		}
		rnk := RouteCompare(url, prt)
		if rnk != 0 {
			if rnk > lrnk {
				lrnk = rnk
				lrt = url
			}
		}
	}
	if lrnk == 0 {
		return nil, NO_MATCH_FOUND
	}
	return func(w http.ResponseWriter, r *http.Request) {
		rt.GetHandlerFunc(lrt.hash)(w, r, lrt.Bind(prt))
	}, nil
}

func (rt RouteTable) GetHandlerFunc(hash string) HandlerFunc {
	return rt.configs[hash]
}
