package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/netio"
	"gopkg.in/yaml.v3"
)

func Parse(in []byte) (Version, *Metadata, any, error) {
	var conf Config
	err := yaml.Unmarshal(in, &conf)
	if err != nil {
		return 0, nil, nil, err
	}
	switch strings.ToLower(conf.APIVersion) {
	case "apps/v1":
		{
			var specs SpecV1
			err := conf.Spec.Decode(&specs)
			if err != nil {
				return 0, nil, nil, err
			}
			return 1, &conf.Metadata, &specs, nil
		}
	}
	return 0, nil, nil, fmt.Errorf("usupported version %s", conf.APIVersion)
}

func ParseV1(resourcesV1 map[string]ResourceV1, handleFunc func(*url.URL, string, string, []netio.Caller)) error {
	for _, value := range resourcesV1 {
		url, err := url.Parse(value.Frontend)
		if err != nil {
			return nil
		}
		callers, err := ParseFiltersV1(value.Filters, true)
		if err != nil {
			return nil
		}
		handleFunc(url, value.Backend, value.Method, callers)
	}
	return nil
}

func ParseFiltersV1(in []FilterV1, supportsLevel bool) ([]netio.Caller, error) {
	callers := make([]netio.Caller, 0)
	for _, caller := range in {
		url, err := url.Parse(caller.Addr)
		if err != nil {
			return nil, err
		}
		filter := filters.NewFilter()
		filter.Address = url
		filter.AwaitList = caller.Await
		filter.Name = caller.Name
		filter.Parallel = caller.Async
		filter.Level = netio.LEVEL_NONE
		if supportsLevel {
			level, err := Level(caller.Level)
			if err != nil {
				return nil, err
			}
			filter.Level = level
		}
		timeout, err := Timeout(caller.Timeout)
		if err != nil {
			return nil, err
		}
		filter.Timeout = timeout
		next, err := ParseFiltersV1(caller.Next, false)
		if err != nil {
			return nil, err
		}
		filter.Callers = next
		c, err := filter.Build()
		if err != nil {
			return nil, err
		}
		callers = append(callers, c)
	}
	return callers, nil
}

func Level(level string) (netio.Level, error) {
	switch strings.ToLower(level) {
	case "connect":
		{
			return netio.LEVEL_CONNECT, nil
		}
	case "request":
		{
			return netio.LEVEL_REQUEST, nil
		}
	case "response":
		{
			return netio.LEVEL_RESPONSE, nil
		}
	}
	return netio.LEVEL_NONE, fmt.Errorf("unsupported level %s", level)
}

func Timeout(str string) (time.Duration, error) {
	if len(str) == 0 {
		return 0, nil
	}
	var buffer bytes.Buffer
	for _, r := range str {
		if !unicode.IsDigit(r) {
			break
		}
		buffer.WriteRune(r)
	}
	unit := str[:buffer.Len()]
	unit = strings.TrimPrefix(unit, " ")
	unit = strings.TrimSuffix(unit, " ")
	n, err := strconv.Atoi(buffer.String())
	if err != nil {
		return 0, err
	}
	switch strings.ToLower(unit) {
	case "ms":
		{
			return time.Millisecond * time.Duration(n), nil
		}
	case "s":
		{
			return time.Second * time.Duration(n), nil
		}
	case "m":
		{
			return time.Minute * time.Duration(n), nil
		}
	case "h":
		{
			return time.Hour * time.Duration(n), nil
		}
	}
	return 0, fmt.Errorf("unsupported unit %s", unit)
}
