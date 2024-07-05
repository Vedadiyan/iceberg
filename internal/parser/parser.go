package parser

import (
	"fmt"
	"strings"

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
			return 1, &conf.Metadata, specs, nil
		}
	}
	return 0, nil, nil, fmt.Errorf("usupported version %s", conf.APIVersion)
}
