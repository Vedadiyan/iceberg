package conf

type (
	OnError string
	Config  struct {
		APIVersion string   `yaml:"apiVersion"`
		Metadata   Metadata `yaml:"metadata"`
		Spec       any      `yaml:"spec"`
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
		Next     []NextV1   `yaml:"next"`
	}
	ExchangeV1 struct {
		Headers []string `yaml:"headers"`
		Body    bool     `yaml:"body"`
	}
	NextV1 struct {
		Name    string   `yaml:"name"`
		Addr    string   `yaml:"addr"`
		OnError OnError  `yaml:"onError"`
		Timeout string   `yaml:"timeout"`
		Async   bool     `yaml:"async"`
		Await   []string `yaml:"await"`
	}
	UseV1 struct {
		Cache CacheV1 `yaml:"cache"`
		Cors  string  `yaml:"cors"`
	}
	CacheV1 struct {
		Addr string `yaml:"addr"`
		TTL  string `yaml:"ttl"`
		Key  string `yaml:"key"`
	}
)

const (
	DEFAULT  OnError = "default"
	TERM     OnError = "term"
	CONTINUE OnError = "continue"
)
