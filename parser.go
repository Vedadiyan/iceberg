package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	auto "github.com/vedadiyan/goal/pkg/config/auto"
	"github.com/vedadiyan/goal/pkg/di"
	"github.com/vedadiyan/iceberg/handlers"
	"gopkg.in/yaml.v3"
)

type (
	ApiVersion int
	Version    struct {
		ApiVersion string `yaml:"apiVersion"`
	}
	SpecV1 struct {
		Listen    string       `yaml:"listen"`
		Resources []ResourceV1 `yaml:"recource"`
	}
	ResourceV1 struct {
		Name         string          `yaml:"name"`
		Frontend     string          `yaml:"frontend"`
		Backend      string          `yaml:"backend"`
		FilterChains []FilterChainV1 `yaml:"filterChain"`
	}
	FilterChainV1 struct {
		Name      string `yaml:"name"`
		Type      string `yaml:"type"`
		Listerner string `yaml:"listener"`
		Level     string `yaml:"level"`
		Method    string `yaml:"method"`
		Conf      ConfV1 `yaml:"conf"`
		Timeout   int    `yaml:"timeout"`
	}
	ConfV1 struct {
		ExchangeHeaders []string `yaml:"exchangeHeaders"`
		ExchangeBody    bool     `yaml:"exchangeBody"`
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

func BuildV1(specV1 *SpecV1) (Server, error) {
	mux := http.NewServeMux()
	for _, resource := range specV1.Resources {
		conf := handlers.Conf{}
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
			switch strings.ToLower(filter.Type) {
			case "http":
				{
					httpFilter := handlers.HttpFilter{}
					httpFilter.Address = url
					httpFilter.ExchangeHeaders = filter.Conf.ExchangeHeaders
					httpFilter.ExchangeBody = filter.Conf.ExchangeBody
					httpFilter.Level = level
					httpFilter.Method = filter.Method
					httpFilter.Timeout = filter.Timeout
					filters = append(filters, &httpFilter)
				}
			case "nats":
				{
					// Safe to use even if multiple registerations takes place
					auto.Register(auto.New[string](url.Scheme, false, func(value string) {
						_ = di.AddScopedWithName[nats.Conn](url.Scheme, func() (instance *nats.Conn, err error) {
							return nats.Connect(value)
						})
					}))
					natsFilter := handlers.NATSFilter{}
					natsFilter.Address = url
					natsFilter.ExchangeHeaders = filter.Conf.ExchangeHeaders
					natsFilter.ExchangeBody = filter.Conf.ExchangeBody
					natsFilter.Level = level
					natsFilter.Timeout = filter.Timeout
					filters = append(filters, &natsFilter)
				}
			}
		}
		conf.Filters = filters
		handler := New(&conf)
		handler(mux)
	}
	return func() error {
		return http.ListenAndServe(specV1.Listen, mux)
	}, nil
}

func Levels(level string) handlers.Level {
	var output handlers.Level
	levels := strings.Split(strings.ToLower(strings.Trim(level, " ")), "|")
	for _, level := range levels {
		switch level {
		case "intercept":
			{
				output = output | handlers.INTERCEPT
			}
		case "post_process":
			{
				output = output | handlers.POST_PROCESS
			}
		case "parallel":
			{
				output = output | handlers.PARALLEL
			}
		}
	}
	return output
}
