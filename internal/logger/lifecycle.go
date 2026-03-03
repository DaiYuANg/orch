package logger

import (
	"github.com/DaiYuANg/toolkit4go/logx"
	"go.uber.org/fx"
)

func deferLogger(lc fx.Lifecycle, logger *logx.Logger) {
	lc.Append(
		fx.StopHook(func() error {
			return logger.Close()
		}),
	)
}
