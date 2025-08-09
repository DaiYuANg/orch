package logger

import (
	"errors"
	"fmt"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"syscall"
)

func deferLogger(lc fx.Lifecycle, logger *zap.Logger) {
	lc.Append(
		fx.StopHook(func() error {
			if err := logger.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
				return fmt.Errorf("logger sync failed: %v", err)
			}
			return nil
		}),
	)
}
