package config

import (
	"log/slog"
	"os"

	"github.com/DaiYuANg/toolkit4go/configx"
	"github.com/DaiYuANg/warden/internal/constant"
	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(
	loadConfig,
))

func loadConfig(logger *slog.Logger) (*Config, error) {
	def := defaultConfig()
	candidates := []string{
		"config.yaml",
		"config.yml",
		"config.toml",
		"config.json",
		"mock/mock.json",
		"mock/mock.yml",
		"mock/mock.toml",
	}
	files := existingFiles(candidates)
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

	logger.Debug("config loaded", "keys", cfg.All())
	return &def, nil
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
