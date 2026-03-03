package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/DaiYuANg/toolkit4go/configx"
	"github.com/DaiYuANg/warden/internal/constant"
	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(
	loadConfig,
))

type loadConfigDependency struct {
	fx.In
	Files []string `name:"conf" optional:"true"`
}

func loadConfig(dep loadConfigDependency) (*Config, error) {
	def := defaultConfig()
	files, err := resolveConfigFiles(dep.Files)
	if err != nil {
		return nil, err
	}

	opts := []configx.Option{
		configx.WithDotenv(),
		configx.WithDefaultsStruct(def),
		configx.WithEnvPrefix(constant.EnvPrefix),
	}
	if len(files) > 0 {
		opts = append(opts, configx.WithFiles(files...))
	}

	cfg, err := configx.LoadConfig(opts...)
	if err != nil {
		return nil, err
	}
	if err := cfg.Unmarshal("", &def); err != nil {
		return nil, err
	}

	slog.Debug("config loaded", "keys", cfg.All(), "files", files)
	return &def, nil
}

func resolveConfigFiles(input []string) ([]string, error) {
	files := compactFiles(input)
	if len(files) > 0 {
		for _, file := range files {
			if _, err := os.Stat(file); err != nil {
				return nil, fmt.Errorf("config file %q: %w", file, err)
			}
		}
		return files, nil
	}

	return existingFiles(defaultConfigCandidates()), nil
}

func defaultConfigCandidates() []string {
	return []string{
		"config.yaml",
		"config.yml",
		"config.toml",
		"config.json",
		"mock/mock.json",
		"mock/mock.yml",
		"mock/mock.toml",
	}
}

func compactFiles(input []string) []string {
	files := make([]string, 0, len(input))
	for _, file := range input {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		files = append(files, trimmed)
	}
	return files
}

func existingFiles(candidates []string) []string {
	files := make([]string, 0, len(candidates))
	for _, file := range candidates {
		if _, err := os.Stat(file); err == nil {
			files = append(files, file)
		}
	}
	return files
}
