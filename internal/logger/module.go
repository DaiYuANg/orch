package logger

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/toolkit4go/logx"
	"github.com/DaiYuANg/warden/internal/config"
	"go.uber.org/fx"
)

var Module = fx.Module("logger_module", fx.Provide(newLogger, newSlogLogger), fx.Invoke(deferLogger))

type NewLoggerDependency struct {
	fx.In
	Config *config.Config `optional:"true"`
}

func newLogger(dep NewLoggerDependency) (*logx.Logger, error) {
	cfg := config.Logger{}
	if dep.Config != nil {
		cfg = dep.Config.Logger
	}

	logPath := filepath.Join(os.TempDir(), "warden.log")
	if cfg.Path != "" {
		logPath = cfg.Path
	}

	level := cfg.Level
	if level == "" {
		level = "debug"
	}
	maxSizeMB := cfg.MaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	maxAgeDays := cfg.MaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = 7
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 5
	}
	console := cfg.Console
	caller := cfg.Caller
	if dep.Config == nil {
		console = true
		caller = true
	}

	return logx.New(
		logx.WithLevelString(level),
		logx.WithConsole(console),
		logx.WithFile(logPath),
		logx.WithFileRotation(maxSizeMB, maxAgeDays, maxBackups),
		logx.WithCaller(caller),
	)
}

func newSlogLogger(log *logx.Logger) *slog.Logger {
	logger := logx.SetDefaultSlog(log)
	return logger
}
