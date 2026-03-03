package http

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DaiYuANg/warden/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/fx"
)

type LifecycleDependency struct {
	fx.In
	Lc     fx.Lifecycle
	App    *fiber.App
	Config *config.Config
	Logger *slog.Logger
	Pool   *ants.Pool
}

func lifecycle(dep LifecycleDependency) {
	lc, app, log, pool, cfg := dep.Lc, dep.App, dep.Logger, dep.Pool, dep.Config
	lc.Append(fx.StartStopHook(
		func() error {
			return pool.Submit(func() {
				localAddress := "http://127.0.0.1:" + cfg.Http.GetPort()
				log.Debug("http listening", "address", localAddress)
				err := app.Listen(
					":" + cfg.Http.GetPort(),
				)
				if err != nil {
					log.Error(fmt.Sprintf("http start fail: %v", err))
				}
			})
		},
		func(ctx context.Context) error {
			return app.ShutdownWithContext(ctx)
		},
	),
	)
}
