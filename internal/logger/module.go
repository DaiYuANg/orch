package logger

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/toolkit4go/logx"
	"go.uber.org/fx"
)

var Module = fx.Module("logger_module", fx.Provide(newLogger, newSlogLogger), fx.Invoke(deferLogger))

type NewLoggerDependency struct {
	fx.In
	FilePath *string `name:"logPath" optional:"true"`
}

func newLogger(dep NewLoggerDependency) (*logx.Logger, error) {
	logPath := filepath.Join(os.TempDir(), "warden.log")
	if dep.FilePath != nil && *dep.FilePath != "" {
		logPath = *dep.FilePath
	}

	return logx.New(
		logx.WithLevel(logx.DebugLevel),
		logx.WithConsole(true),
		logx.WithFile(logPath),
		logx.WithFileRotation(100, 7, 5),
		logx.WithCaller(true),
	)
}

func newSlogLogger(log *logx.Logger) *slog.Logger {
	logger := logx.SetDefaultSlog(log)
	return logger
}
