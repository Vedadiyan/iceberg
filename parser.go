package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type (
	Version struct {
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

func Parse(file string) (any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	version := Version{}
	err = yaml.Unmarshal(data, &version)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(version.ApiVersion) {
	case "iceberg/v1":
		{
			var specV1 SpecV1
			err := yaml.Unmarshal(data, &specV1)
			if err != nil {
				return nil, err
			}
			return specV1, nil
		}
	default:
		{
			return nil, fmt.Errorf("unsupported case")
		}
	}
}
