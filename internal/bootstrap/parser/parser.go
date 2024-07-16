package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/vedadiyan/iceberg/internal/callers/filters"
	"github.com/vedadiyan/iceberg/internal/common/netio"
	"github.com/vedadiyan/iceberg/internal/middleware/opa"
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
		middleware := make([]netio.Caller, 0)
		if value.Use.OPA != nil {
			url, err := url.Parse(value.Use.OPA.Agent)
			if err != nil {
				return err
			}
			httpPolicies, err := ParsePolicy(value.Use.OPA.Http)
			if err != nil {
				return err
			}
			sendPolicies, err := ParsePolicy(value.Use.OPA.WS.Send)
			if err != nil {
				return err
			}
			receivePolicy, err := ParsePolicy(value.Use.OPA.WS.Receive)
			if err != nil {
				return err
			}
			http, err := opa.NewOpaNats(&opa.Opa{
				AppName:  "",
				Agent:    url,
				Policies: httpPolicies,
				Type:     opa.OPA_TYPE_HTTP,
				Timeout:  time.Second * 30,
			})
			if err != nil {
				return err
			}
			send, err := opa.NewOpaNats(&opa.Opa{
				AppName:  "",
				Agent:    url,
				Policies: sendPolicies,
				Type:     opa.OPA_TYPE_WS_SEND,
				Timeout:  time.Second * 30,
			})
			if err != nil {
				return err
			}
			receive, err := opa.NewOpaNats(&opa.Opa{
				AppName:  "",
				Agent:    url,
				Policies: receivePolicy,
				Type:     opa.OPA_TYPE_WS_RECEIVE,
				Timeout:  time.Second * 30,
			})
			if err != nil {
				return err
			}
			middleware = append(middleware, http)
			middleware = append(middleware, send)
			middleware = append(middleware, receive)
		}
		middleware = append(middleware, callers...)
		handleFunc(url, value.Backend, value.Method, middleware)
	}
	return nil
}

func ParsePolicy(in []any) (map[string]opa.PolicyType, error) {
	policies := make(map[string]opa.PolicyType)
	for _, item := range in {
		switch item := item.(type) {
		case map[string]any:
			{
				for key, value := range item {
					val, ok := value.(string)
					if !ok {
						return nil, fmt.Errorf("expected string but found %T", value)
					}
					switch strings.ToLower(val) {
					case "local":
						{
							policies[key] = opa.POLICY_TYPE_LOCAL
						}
					case "remote":
						{
							policies[key] = opa.POLICY_TYPE_REMOTE
						}
					default:
						{
							return nil, fmt.Errorf("unsupported value %s", val)
						}
					}
					break
				}
			}
		case string:
			{
				policies[item] = opa.POLICY_TYPE_REMOTE
			}
		}
	}
	return policies, nil
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
