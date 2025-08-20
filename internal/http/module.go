package http

import (
	"github.com/DaiYuANg/warden/internal/config"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("http",
	fx.Provide(
		newFiber,
		newOpenapi,
	),
	middleware,
	socketIOModule,
	jwt,
	fx.Invoke(
		lifecycle,
	),
)

func newFiber() *fiber.App {
	app := fiber.New(
		fiber.Config{
			DisableStartupMessage: true,
			ReduceMemoryUsage:     true,
			EnablePrintRoutes:     false,
		},
	)

	return app
}

func newOpenapi(app *fiber.App, config *config.Config) huma.API {
	humaCfg := huma.DefaultConfig("warden", "0.1")
	humaCfg.Servers = []*huma.Server{{
		URL: "http://localhost:" + config.Http.GetPort(),
	}, {
		URL: "http://127.0.0.1:" + config.Http.GetPort(),
	}}
	humaCfg.DocsPath = "/"
	return humafiber.New(app, humaCfg)
}
