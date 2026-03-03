package config

import "strconv"

type Http struct {
	Port                  int    `koanf:"port"`
	APIDoc                APIDoc `koanf:"api_doc"`
	DisableStartupMessage bool   `koanf:"disable_startup_message"`
	ReduceMemoryUsage     bool   `koanf:"reduce_memory_usage"`
	PrintRoutes           bool   `koanf:"print_routes"`
}

type APIDoc struct {
	Enable      bool   `koanf:"enable"`
	Title       string `koanf:"title"`
	Version     string `koanf:"version"`
	Description string `koanf:"description"`
	Path        string `koanf:"path"`
	OpenAPIPath string `koanf:"openapi_path"`
}

func (h Http) GetPort() string {
	return strconv.Itoa(h.Port)
}

type Logger struct {
	Level      string `koanf:"level"`
	Console    bool   `koanf:"console"`
	Caller     bool   `koanf:"caller"`
	Path       string `koanf:"path"`
	MaxSizeMB  int    `koanf:"max_size_mb"`
	MaxAgeDays int    `koanf:"max_age_days"`
	MaxBackups int    `koanf:"max_backups"`
}
