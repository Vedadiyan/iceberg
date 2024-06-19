package parsers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/internal/common"
	"github.com/vedadiyan/iceberg/internal/filters"
	"github.com/vedadiyan/iceberg/internal/logger"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type (
	ApiVersion int
	Version    struct {
		ApiVersion string `yaml:"apiVersion"`
	}
	SpecV1 struct {
		Spec struct {
			AppName   string       `yaml:"appName"`
			Listen    string       `yaml:"listen"`
			Configs   ConfigsV1    `yaml:"configs"`
			Resources []ResourceV1 `yaml:"resources"`
		} `yaml:"spec"`
	}
	ConfigsV1 struct {
		CORS any `yaml:"cors"`
	}
	ResourceV1 struct {
		Name         string          `yaml:"name"`
		Frontend     string          `yaml:"frontend"`
		Backend      string          `yaml:"backend"`
		Auth         *AuthV1         `yaml:"auth"`
		FilterChains []FilterChainV1 `yaml:"filterChains"`
	}
	FilterChainV1 struct {
		Name      string         `yaml:"name"`
		Listerner string         `yaml:"listener"`
		Level     string         `yaml:"level"`
		Method    string         `yaml:"method"`
		Exchange  ExchangeV1     `yaml:"exchange"`
		Callbacks []CallbackV1   `yaml:"callbacks"`
		Headers   map[string]any `yaml:"headers"`
		Timeout   int            `yaml:"timeout"`
		Await     []string       `yaml:"await"`
		Durable   bool           `yaml:"durable"`
	}
	AuthV1 struct {
		Listerner string `yaml:"listener"`
		Method    string `yaml:"method"`
		Timeout   int    `yaml:"timeout"`
	}
	CallbackV1 struct {
		Name      string       `yaml:"name"`
		Listerner string       `yaml:"listener"`
		Parallel  bool         `yaml:"parallel"`
		Method    string       `yaml:"method"`
		Timeout   int          `yaml:"timeout"`
		Durable   bool         `yaml:"durable"`
		Await     []string     `yaml:"await"`
		Callbacks []CallbackV1 `yaml:"callbacks"`
	}
	ExchangeV1 struct {
		Headers []string `yaml:"headers"`
		Body    bool     `yaml:"body"`
	}
	Server func() error
)

const (
	VER_NONE ApiVersion = iota
	VER_V1
)

var (
	_initializers []func() error
)

func init() {
	_initializers = make([]func() error, 0)
}

func Parse() (ApiVersion, any, error) {
	data := os.Getenv("ICEBERG_CONFIG")
	if len(data) == 0 {
		return VER_NONE, nil, fmt.Errorf("iceberg config not found")
	}
	version := Version{}
	err := yaml.Unmarshal([]byte(data), &version)
	if err != nil {
		return VER_NONE, nil, err
	}
	switch strings.ToLower(version.ApiVersion) {
	case "iceberg/v1":
		{
			var specV1 SpecV1
			err := yaml.Unmarshal([]byte(data), &specV1)
			if err != nil {
				return VER_NONE, nil, err
			}
			return VER_V1, &specV1, nil
		}
	default:
		{
			return VER_NONE, nil, fmt.Errorf("unsupported case")
		}
	}
}

func GetCORSOptions(specV1 *SpecV1) (*filters.CORS, error) {
	switch value := specV1.Spec.Configs.CORS.(type) {
	case string:
		{
			if strings.ToLower(value) != "default" {
				return nil, fmt.Errorf("unexpected value %s", value)
			}
			cors := &filters.CORS{}
			cors.Origins = "*"
			cors.Headers = "*"
			cors.Methods = "GET, DELETE, OPTIONS, POST, PUT"
			cors.ExposeHeader = "*"
			cors.MaxAge = "3628800"
			return cors, nil
		}
	case map[string]any:
		{
			cors := &filters.CORS{}
			for key, value := range value {
				switch strings.ToLower(key) {
				case "origin":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.Origins = *value
					}
				case "methods":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.Methods = *value
					}
				case "headersallowed":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.Headers = *value
					}
				case "headersexposed":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.ExposeHeader = *value
					}
				case "maxage":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.MaxAge = *value
					}
				}
			}
			return cors, nil
		}
	default:
		{
			return nil, nil
		}
	}
}

func GetCORSValue[T any](key string, value any) (*T, error) {
	v, ok := value.(T)
	if !ok {
		return nil, fmt.Errorf("%s: expected string but found %T", key, value)
	}
	return &v, nil
}

func BuildV1(specV1 *SpecV1, handlerFunc func(conf *filters.Conf) common.Handler) (Server, error) {
	mux := http.NewServeMux()
	cors, err := GetCORSOptions(specV1)
	if err != nil {
		return nil, err
	}
	for _, resource := range specV1.Spec.Resources {
		conf := filters.Conf{}
		conf.CORS = cors
		backendUrl, err := url.Parse(resource.Backend)
		if err != nil {
			return nil, err
		}
		frontendUrl, err := url.Parse(resource.Frontend)
		if err != nil {
			return nil, err
		}
		conf.Backend = backendUrl
		conf.Frontend = frontendUrl
		filters, err := BuildFilterChainV1s(specV1.Spec.AppName, resource.FilterChains)
		if err != nil {
			return nil, err
		}
		auth, err := BuildAuthV1(specV1.Spec.AppName, resource.Auth)
		if err != nil {
			return nil, err
		}
		logger.Info("config parsed")
		conf.Filters = filters
		conf.Auth = auth
		handler := handlerFunc(&conf)
		logger.Info("config", conf)
		handler(mux)
	}
	return func() error {
		for _, initializer := range _initializers {
			err := initializer()
			if err != nil {
				return err
			}
		}
		return http.ListenAndServe(specV1.Spec.Listen, mux)
	}, nil
}

func BuildFilterChainV1s(appName string, filterChains []FilterChainV1) ([]filters.Filter, error) {
	filterList := make([]filters.Filter, 0)
	for _, filter := range filterChains {
		filter, err := BuildFilterChainV1(appName, filter)
		if err != nil {
			return nil, err
		}
		filterList = append(filterList, filter)
	}
	return filterList, nil
}

func BuildFilterChainV1(appName string, filter FilterChainV1) (filters.Filter, error) {
	url, err := url.Parse(filter.Listerner)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(url.Scheme) {
	case "http", "https":
		{
			return BuildHttp(appName, filter, url)
		}
	case "nats":
		{
			return BuildNats(appName, filter, url)
		}
	case "grpc":
		{
			return BuildGrpc(appName, filter, url)
		}
	}
	return nil, fmt.Errorf("invalid filter")
}

func BuildAuthV1(appName string, filter *AuthV1) (filters.Filter, error) {
	if filter == nil {
		return nil, nil
	}
	url, err := url.Parse(filter.Listerner)
	if err != nil {
		return nil, err
	}

	f := FilterChainV1{}
	f.Name = "Authentication"
	f.Level = "request"
	f.Method = filter.Method

	switch strings.ToLower(url.Scheme) {
	case "http", "https":
		{
			return BuildHttp(appName, f, url)
		}
	case "nats":
		{
			return BuildNats(appName, f, url)
		}
	case "grpc":
		{
			return BuildGrpc(appName, f, url)
		}
	}
	return nil, fmt.Errorf("invalid filter")
}

func BuildCallbacksV1(appName string, callbacks []CallbackV1) ([]filters.Filter, error) {
	output := make([]filters.Filter, 0)
	for _, filter := range callbacks {
		level := "inherit"
		if filter.Parallel {
			level += "|parallel"
		}
		callback, err := BuildFilterChainV1(appName, FilterChainV1{
			Name:      filter.Name,
			Listerner: filter.Listerner,
			Method:    filter.Method,
			Level:     level,
			Durable:   filter.Durable,
			Callbacks: filter.Callbacks,
			Await:     filter.Await,
		})
		if err != nil {
			return nil, err
		}
		output = append(output, callback)
	}
	return output, nil
}

func BuildHttp(appName string, filter FilterChainV1, url *url.URL) (filters.Filter, error) {
	callbacks, err := BuildCallbacksV1(appName, filter.Callbacks)
	if err != nil {
		return nil, err
	}
	level := Levels(filter.Level)
	if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
		host := strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
		auto.Register(auto.New(host, false, func(value string) {
			url.Host = value
		}))
	}
	httpFilter := filters.HttpFilter{}
	httpFilter.Name = filter.Name
	httpFilter.Address = url
	httpFilter.ExchangeHeaders = filter.Exchange.Headers
	httpFilter.ExchangeBody = filter.Exchange.Body
	httpFilter.Level = level
	httpFilter.Method = filter.Method
	httpFilter.Timeout = filter.Timeout
	httpFilter.Filters = callbacks
	httpFilter.AwaitList = filter.Await
	return &httpFilter, nil
}

func BuildNats(appName string, filter FilterChainV1, url *url.URL) (filters.Filter, error) {
	callbacks, err := BuildCallbacksV1(appName, filter.Callbacks)
	if err != nil {
		return nil, err
	}
	level := Levels(filter.Level)
	if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
		url.Host = strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
		auto.Register(auto.New(url.Host, false, func(value string) {
			logger.Info("nats", value)
			_ = di.AddSinletonWithName(url.Host, func() (instance *nats.Conn, err error) {
				return nats.Connect(value)
			})
		}))
	} else {
		_ = di.AddSinletonWithName(url.Host, func() (instance *nats.Conn, err error) {
			return nats.Connect(url.Host)
		})
	}
	subject := strings.TrimPrefix(url.Path, "/")
	natsFilter := filters.NATSFilter{}
	natsFilter.Name = filter.Name
	natsFilter.Address = url
	natsFilter.ExchangeHeaders = filter.Exchange.Headers
	natsFilter.ExchangeBody = filter.Exchange.Body
	natsFilter.Level = level
	natsFilter.Timeout = filter.Timeout
	natsFilter.Url = url.Host
	natsFilter.Filters = callbacks
	natsFilter.Subject = subject
	natsFilter.AwaitList = filter.Await
	natsFilter.Durable = filter.Durable
	natsFilter.ReflectionKey = fmt.Sprintf("$ICERBERG_%s_%s", strings.ToUpper(appName), strings.ToUpper(subject))
	_initializers = append(_initializers, natsFilter.InitializeReflector)
	return &natsFilter, nil
}

func BuildGrpc(appName string, filter FilterChainV1, url *url.URL) (filters.Filter, error) {
	callbacks, err := BuildCallbacksV1(appName, filter.Callbacks)
	if err != nil {
		return nil, err
	}
	level := Levels(filter.Level)
	if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
		url.Host = strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
		auto.Register(auto.New(url.Host, false, func(value string) {
			_ = di.AddSinletonWithName(url.Host, func() (instance *grpc.ClientConn, err error) {
				return grpc.Dial(value)
			})
		}))
	} else {
		_ = di.AddSinletonWithName(url.Host, func() (instance *grpc.ClientConn, err error) {
			return grpc.Dial(url.Host)
		})
	}
	grpcFilter := filters.GRPCFilter{}
	grpcFilter.Name = filter.Name
	grpcFilter.Address = url
	grpcFilter.ExchangeHeaders = filter.Exchange.Headers
	grpcFilter.ExchangeBody = filter.Exchange.Body
	grpcFilter.Level = level
	grpcFilter.Timeout = filter.Timeout
	grpcFilter.Url = url.Host
	grpcFilter.Subject = strings.TrimPrefix(url.Path, "/")
	grpcFilter.Filters = callbacks
	grpcFilter.AwaitList = filter.Await
	return &grpcFilter, nil
}

func Levels(level string) filters.Level {
	var output filters.Level
	levels := strings.Split(strings.ToLower(strings.Trim(level, " ")), "|")
	for _, level := range levels {
		switch level {
		case "inherit":
			{
				output = output | filters.INHERIT
			}
		case "request":
			{
				output = output | filters.REQUEST
			}
		case "response":
			{
				output = output | filters.RESPONSE
			}
		case "parallel":
			{
				output = output | filters.PARALLEL
			}
		}
	}
	return output
}
