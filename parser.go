package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
		Conf      ConfV1 `yaml:"conf"`
	}
	ConfV1 struct {
		ExchangeHeaders []string `yaml:"exchangeHeaders"`
		ExchangeBody    bool     `yaml:"exchangeBody"`
	}
)

const (
	VER_NONE ApiVersion = iota
	VER_V1
)

func Parse(file string) (ApiVersion, any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return VER_NONE, nil, err
	}
	version := Version{}
	err = yaml.Unmarshal(data, &version)
	if err != nil {
		return VER_NONE, nil, err
	}
	switch strings.ToLower(version.ApiVersion) {
	case "iceberg/v1":
		{
			var specV1 SpecV1
			err := yaml.Unmarshal(data, &specV1)
			if err != nil {
				return VER_NONE, nil, err
			}
			return VER_V1, specV1, nil
		}
	default:
		{
			return VER_NONE, nil, fmt.Errorf("unsupported case")
		}
	}
}

func BuildV1(specV1 *SpecV1) (func() error, error) {
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
			switch strings.ToLower(filter.Type) {
			case "http":
				{
					url, err := url.Parse(filter.Listerner)
					if err != nil {
						return nil, err
					}
					f := handlers.HttpFilter{}
					f.Address = url
					f.ExchangeHeaders = filter.Conf.ExchangeHeaders
					f.ExchangeBody = filter.Conf.ExchangeBody
					filters = append(filters, &f)
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
