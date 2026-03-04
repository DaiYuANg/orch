package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/DaiYuANg/toolkit4go/configx"
	"github.com/DaiYuANg/warden/internal/constant"
	"github.com/samber/lo"
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
		type fileStatError struct {
			file string
			err  error
		}
		statErrors := lo.FilterMap(files, func(file string, _ int) (fileStatError, bool) {
			_, err := os.Stat(file)
			if err == nil {
				return fileStatError{}, false
			}
			return fileStatError{file: file, err: err}, true
		})
		if len(statErrors) > 0 {
			first := statErrors[0]
			return nil, fmt.Errorf("config file %q: %w", first.file, first.err)
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
	return lo.FilterMap(input, func(file string, _ int) (string, bool) {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	})
}

func existingFiles(candidates []string) []string {
	return lo.Filter(candidates, func(file string, _ int) bool {
		_, err := os.Stat(file)
		return err == nil
	})
}
