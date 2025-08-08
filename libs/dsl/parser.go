package dsl

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"gopkg.in/yaml.v3"
)

type Parser struct {
}

func ParseYAML(path string) (*Workload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var w Workload
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, err
	}

	return &w, nil
}

func ParseHCL(path string) (*Workload, error) {
	var w Workload
	err := hclsimple.DecodeFile(path, nil, &w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
