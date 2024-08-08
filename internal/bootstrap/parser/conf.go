package parser

import "gopkg.in/yaml.v3"

type (
	OnError string
	Version int
	Config  struct {
		APIVersion string    `yaml:"apiVersion"`
		Metadata   Metadata  `yaml:"metadata"`
		Spec       yaml.Node `yaml:"spec"`
	}
	Metadata struct {
		Name string `yaml:"name"`
	}
	SpecV1 struct {
		Listen    string                `yaml:"listen"`
		Resources map[string]ResourceV1 `yaml:"resources"`
	}
	ResourceV1 struct {
		Frontend string     `yaml:"frontend"`
		Backend  string     `yaml:"backend"`
		Method   string     `yaml:"method"`
		Use      UseV1      `yaml:"use"`
		Filters  []FilterV1 `yaml:"filters"`
	}
	FilterV1 struct {
		Name     string     `yaml:"name"`
		Addr     string     `yaml:"addr"`
		Level    string     `yaml:"level"`
		Timeout  string     `yaml:"timeout"`
		OnError  OnError    `yaml:"onError"`
		Async    bool       `yaml:"async"`
		Await    []string   `yaml:"await"`
		Exchange ExchangeV1 `yaml:"exchange"`
		Next     []FilterV1 `yaml:"next"`
	}
	ExchangeV1 struct {
		Headers []string `yaml:"headers"`
		Body    bool     `yaml:"body"`
	}
	UseV1 struct {
		Cache *CacheV1 `yaml:"cache"`
		Cors  *string  `yaml:"cors"`
		OPA   *OpaV1   `yaml:"opa"`
		Log   *LogV1   `yaml:"log"`
	}
	CacheV1 struct {
		Addr string `yaml:"addr"`
		TTL  string `yaml:"ttl"`
		Key  string `yaml:"key"`
	}
	OpaV1 struct {
		Agent string  `yaml:"agent"`
		Http  []any   `yaml:"http"`
		WS    OpaWSV1 `yaml:"ws"`
	}
	OpaWSV1 struct {
		Send    []any `yaml:"send"`
		Receive []any `yaml:"receive"`
	}
	LogV1 struct {
		Agent     string     `yaml:"agent"`
		LogLevel  string     `yaml:"logLevel"`
		Batch     LogBatchV1 `yaml:"batch"`
		Fallbacks []string   `yaml:"fallbacks"`
		Retry     LogRetryV1 `yaml:"retry"`
	}
	LogBatchV1 struct {
		MinBufferSize int    `yaml:"minBufferSize"`
		Interval      string `yaml:"interval"`
	}
	LogRetryV1 struct {
		Max   int    `yaml:"max"`
		Pause string `yaml:"pause"`
	}
)

const (
	DEFAULT  OnError = "default"
	TERM     OnError = "term"
	CONTINUE OnError = "continue"
)
