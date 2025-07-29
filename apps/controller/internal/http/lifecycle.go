package http

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type LifecycleDependency struct {
	fx.In
	Lc  fx.Lifecycle
	App *fiber.App
	//Config *config.Config
	Logger *zap.SugaredLogger
	Pool   *ants.Pool
}

func lifecycle(dep LifecycleDependency) {
	lc, app, log, pool := dep.Lc, dep.App, dep.Logger, dep.Pool
	lc.Append(fx.StartStopHook(
		func() error {
			return pool.Submit(func() {
				//localAddress := "http://127.0.0.1:" + cfg.Http.GetPort()
				//log.Debugf("Http Listening on %s", localAddress)
				err := app.Listen(
					":3000",
					//fiber.ListenConfig{
					//	DisableStartupMessage: true,
					//	EnablePrintRoutes:     false,
					//	EnablePrefork:         false,
					//	ShutdownTimeout:       1000,
					//},
				)
				if err != nil {
					log.Errorf("spack start fail: %v", err)
				}
			})
		},
		func(ctx context.Context) error {
			return app.ShutdownWithContext(ctx)
		},
	),
	)
}
