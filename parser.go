package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/handlers"
	"github.com/vedadiyan/natsch"
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
		FilterChains []FilterChainV1 `yaml:"filterChains"`
	}
	FilterChainV1 struct {
		Name      string     `yaml:"name"`
		Listerner string     `yaml:"listener"`
		Level     string     `yaml:"level"`
		Method    string     `yaml:"method"`
		Exchange  ExchangeV1 `yaml:"exchange"`
		Timeout   int        `yaml:"timeout"`
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

func GetCORSOptions(specV1 *SpecV1) (*handlers.CORS, error) {
	switch value := specV1.Spec.Configs.CORS.(type) {
	case string:
		{
			if strings.ToLower(value) != "default" {
				return nil, fmt.Errorf("unexpected value %s", value)
			}
			cors := &handlers.CORS{}
			cors.Origins = "*"
			cors.Headers = "*"
			cors.Methods = "GET, DELETE, OPTIONS, POST, PUT"
			cors.ExposeHeader = "*"
			cors.MaxAge = "3628800"
			return cors, nil
		}
	case map[string]any:
		{
			cors := &handlers.CORS{}
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
				case "headersAllowed":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.Headers = *value
					}
				case "headersExposed":
					{
						value, err := GetCORSValue[string](key, value)
						if err != nil {
							return nil, err
						}
						cors.ExposeHeader = *value
					}
				case "maxAge":
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

func BuildV1(specV1 *SpecV1) (Server, error) {
	mux := http.NewServeMux()
	cors, err := GetCORSOptions(specV1)
	if err != nil {
		return nil, err
	}
	for _, resource := range specV1.Spec.Resources {
		conf := handlers.Conf{}
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
		filters := make([]handlers.Filter, 0)
		for _, filter := range resource.FilterChains {
			url, err := url.Parse(filter.Listerner)
			if err != nil {
				return nil, err
			}
			level := Levels(filter.Level)
			switch strings.ToLower(url.Scheme) {
			case "http", "https":
				{
					if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
						host := strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
						auto.Register(auto.New[string](host, false, func(value string) {
							url.Host = value
						}))
					}
					httpFilter := handlers.HttpFilter{}
					httpFilter.Address = url
					httpFilter.ExchangeHeaders = filter.Exchange.Headers
					httpFilter.ExchangeBody = filter.Exchange.Body
					httpFilter.Level = level
					httpFilter.Method = filter.Method
					httpFilter.Timeout = filter.Timeout
					filters = append(filters, &httpFilter)
				}
			case "nats":
				{
					if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
						url.Host = strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
						auto.Register(auto.New[string](url.Host, false, func(value string) {
							log.Println("nats:", value)
							_ = di.AddSinletonWithName[nats.Conn](url.Host, func() (instance *nats.Conn, err error) {
								return nats.Connect(value)
							})
						}))
					} else {
						_ = di.AddSinletonWithName[nats.Conn](url.Host, func() (instance *nats.Conn, err error) {
							return nats.Connect(url.Host)
						})
					}
					natsFilter := handlers.NATSFilter{}
					natsFilter.Address = url
					natsFilter.ExchangeHeaders = filter.Exchange.Headers
					natsFilter.ExchangeBody = filter.Exchange.Body
					natsFilter.Level = level
					natsFilter.Timeout = filter.Timeout
					natsFilter.Url = url.Host
					natsFilter.Subject = strings.TrimPrefix(url.Path, "/")
					filters = append(filters, &natsFilter)
				}
			case "natsch":
				{
					segments := strings.Split(url.Opaque, "://")
					if len(segments) == 1 {
						segments = append(segments, segments[0])
						segments[0] = "0"
					}
					deadlineTimestamp, err := strconv.ParseInt(segments[0], 10, 64)
					if err != nil {
						return nil, err
					}
					url, err = url.Parse(fmt.Sprintf("natsch://%s", segments[1]))
					if err != nil {
						return nil, err
					}
					if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
						url.Host = strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
						auto.Register(auto.New[string](url.Host, false, func(value string) {
							log.Println("natsch:", value)
							_ = di.AddSinletonWithName[natsch.Conn](url.Host, func() (instance *natsch.Conn, err error) {
								conn, err := nats.Connect(value)
								if err != nil {
									return nil, err
								}
								return natsch.New(conn)
							})
						}))
					} else {
						_ = di.AddSinletonWithName[nats.Conn](url.Host, func() (instance *nats.Conn, err error) {
							return nats.Connect(url.Host)
						})
					}
					natsFilter := handlers.NATSCHFilter{}
					natsFilter.Address = url
					natsFilter.ExchangeHeaders = filter.Exchange.Headers
					natsFilter.ExchangeBody = filter.Exchange.Body
					natsFilter.Level = level
					natsFilter.Timeout = filter.Timeout
					natsFilter.Url = url.Host
					natsFilter.Deadline = time.UnixMicro(deadlineTimestamp)
					natsFilter.Subject = strings.TrimPrefix(url.Path, "/")
					filters = append(filters, &natsFilter)
				}
			case "grpc":
				{
					if strings.HasPrefix(url.Host, "[[") && strings.HasSuffix(url.Host, "]]") {
						url.Host = strings.TrimSuffix(strings.TrimPrefix(url.Host, "[["), "]]")
						auto.Register(auto.New[string](url.Host, false, func(value string) {
							_ = di.AddSinletonWithName[grpc.ClientConn](url.Host, func() (instance *grpc.ClientConn, err error) {
								return grpc.Dial(value)
							})
						}))
					} else {
						_ = di.AddSinletonWithName[grpc.ClientConn](url.Host, func() (instance *grpc.ClientConn, err error) {
							return grpc.Dial(url.Host)
						})
					}
					grpcFilter := handlers.GRPCFilter{}
					grpcFilter.Address = url
					grpcFilter.ExchangeHeaders = filter.Exchange.Headers
					grpcFilter.ExchangeBody = filter.Exchange.Body
					grpcFilter.Level = level
					grpcFilter.Timeout = filter.Timeout
					grpcFilter.Url = url.Host
					grpcFilter.Subject = strings.TrimPrefix(url.Path, "/")
					filters = append(filters, &grpcFilter)
				}
			}
		}
		log.Println("config parsed")
		conf.Filters = filters
		handler := New(&conf)
		log.Println(conf)
		handler(mux)
	}
	return func() error {
		return http.ListenAndServe(specV1.Spec.Listen, mux)
	}, nil
}

func Levels(level string) handlers.Level {
	var output handlers.Level
	levels := strings.Split(strings.ToLower(strings.Trim(level, " ")), "|")
	for _, level := range levels {
		switch level {
		case "request":
			{
				output = output | handlers.REQUEST
			}
		case "response":
			{
				output = output | handlers.RESPONSE
			}
		case "parallel":
			{
				output = output | handlers.PARALLEL
			}
		}
	}
	return output
}
