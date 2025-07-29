package cmd

import (
	"github.com/DaiYuANg/warden/agent/internal/schedule"
	"github.com/DaiYuANg/warden/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func container() *fx.App {
	return fx.New(
		logger.Module,
		schedule.Module,
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			fxLogger := &fxevent.ZapLogger{Logger: log}
			fxLogger.UseLogLevel(zapcore.DebugLevel)
			return fxLogger
		}),
	)
}
