package task

import (
	"gopkg.in/yaml.v3"
)

func parse(body []byte) error {
	var config Config
	err := yaml.Unmarshal(body, &config)
	if err != nil {
		return err
	}
	return nil
}
