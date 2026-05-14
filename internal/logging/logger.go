package logging

import (
	"log/slog"

	"github.com/arcgolabs/logx"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func New(cfg config.LogConfig) (*slog.Logger, error) {
	level, err := logx.ParseLevel(cfg.Level)
	if err != nil {
		level = slog.LevelInfo
	}

	logger, err := logx.New(
		logx.WithConsole(true),
		logx.WithCaller(true),
		logx.WithLevel(level),
	)
	if err != nil {
		return nil, oopsx.B("logging").Wrapf(err, "build logger")
	}
	return logger, nil
}
