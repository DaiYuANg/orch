package config

import (
	"github.com/DaiYuANg/warden/controller/internal/constant"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"strings"
)

var Module = fx.Module("config", fx.Provide(
	newKoanf,
	loadConfig,
))

func newKoanf() *koanf.Koanf {
	return koanf.New(".")
}

func loadConfig(k *koanf.Koanf, logger *zap.SugaredLogger) (*Config, error) {
	def := defaultConfig()

	// 加载默认配置
	if err := k.Load(structs.Provider(def, "koanf"), nil); err != nil {
		return nil, err
	}
	// Load JSON config.
	if err := k.Load(file.Provider("mock/mock.json"), json.Parser()); err != nil {
		logger.Warnf("error loading config: %v", err)
	}

	// Load YAML config and merge into the previously loaded config (because we can).
	err := k.Load(file.Provider("mock/mock.yml"), yaml.Parser())
	if err != nil {
		logger.Warnf("error loading config: %v", err)
	}
	err = k.Load(file.Provider("mock/mock.toml"), toml.Parser())
	if err != nil {
		logger.Warnf("error loading config: %v", err)
	}

	// 使用 lo.Ternary 优化字符串映射函数
	mapEnvKey := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, constant.EnvPrefix)), "_", ".")
	}
	if err := k.Load(env.Provider(constant.EnvPrefix, ".", mapEnvKey), nil); err != nil {
		return nil, err
	}

	allKeys := k.All()
	logger.Debugf("all key: %v", allKeys)

	if err := k.Unmarshal("", &def); err != nil {
		return nil, err
	}

	logger.Infof("loaded config: %+v", def)
	return &def, nil
}
