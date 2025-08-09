package config

import "strconv"

type Http struct {
	Port int `koanf:"port"`
}

type APIDoc struct {
	Enable bool   `koanf:"enable"`
	Path   string `koanf:"path"`
}

func (h Http) GetPort() string {
	return strconv.Itoa(h.Port)
}

type Logger struct {
	Level string `koanf:"level"`
}
