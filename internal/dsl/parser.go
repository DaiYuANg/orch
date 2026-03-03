package dsl

import (
	"errors"
	"fmt"
	"os"
	"strings"

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

	return ParseYAMLContent(data)
}

func ParseYAMLContent(data []byte) (*Workload, error) {
	var w Workload
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	w.normalize()

	return &w, nil
}

func ParseHCL(path string) (*Workload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseHCLContent(data)
}

func ParseHCLContent(data []byte) (*Workload, error) {
	var w Workload
	err := hclsimple.Decode("inline.hcl", data, nil, &w)
	if err != nil {
		return nil, err
	}
	w.normalize()
	return &w, nil
}

func ParseContent(format string, data []byte) (*Workload, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return ParseYAMLContent(data)
	case "hcl":
		return ParseHCLContent(data)
	default:
		return nil, fmt.Errorf("unsupported dsl format: %s", format)
	}
}

func DetectFormat(filename, formatHint string) (string, error) {
	hint := strings.ToLower(strings.TrimSpace(formatHint))
	if hint != "" {
		switch hint {
		case "yaml", "yml":
			return "yaml", nil
		case "hcl":
			return "hcl", nil
		default:
			return "", fmt.Errorf("unsupported dsl format hint: %s", formatHint)
		}
	}

	ext := strings.ToLower(strings.TrimSpace(filename))
	if strings.HasSuffix(ext, ".yaml") || strings.HasSuffix(ext, ".yml") {
		return "yaml", nil
	}
	if strings.HasSuffix(ext, ".hcl") {
		return "hcl", nil
	}
	return "", errors.New("cannot detect dsl format, use format field: yaml|hcl")
}
