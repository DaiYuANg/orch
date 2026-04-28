package logging

import (
	"log/slog"

	"github.com/arcgolabs/logx"

	"github.com/daiyuang/orch/internal/config"
)

func New(cfg config.LogConfig) (*slog.Logger, error) {
	level, err := logx.ParseLevel(cfg.Level)
	if err != nil {
		level = slog.LevelInfo
	}

	return logx.New(
		logx.WithConsole(true),
		logx.WithCaller(true),
		logx.WithLevel(level),
	)
}

