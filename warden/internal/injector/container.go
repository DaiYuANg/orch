package injector

import (
	"github.com/DaiYuANg/warden/internal/logger"
	"github.com/DaiYuANg/warden/internal/schedule"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var commonOption = fx.Options(
	logger.Module,
	schedule.Module,
	fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
		fxLogger := &fxevent.ZapLogger{Logger: log}
		fxLogger.UseLogLevel(zapcore.DebugLevel)
		return fxLogger
	}),
)

func CreateContainer(option ...fx.Option) *fx.App {
	app := fx.New(
		commonOption,
		fx.Options(option...),
	)

	return app
}
