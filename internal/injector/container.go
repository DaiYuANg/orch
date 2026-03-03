package injector

import (
	"log/slog"

	"github.com/DaiYuANg/warden/internal/logger"
	"github.com/DaiYuANg/warden/internal/schedule"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

var commonOption = fx.Options(
	logger.Module,
	schedule.Module,
	fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
		return &fxevent.SlogLogger{Logger: log}
	}),
)

func CreateContainer(option ...fx.Option) (*fx.App, error) {
	options := append(option, commonOption)
	err := fx.ValidateApp(options...)
	if err != nil {
		return nil, err
	}
	app := fx.New(
		commonOption,
		fx.Options(option...),
	)

	return app, nil
}
