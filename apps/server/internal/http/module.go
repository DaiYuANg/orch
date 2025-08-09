package http

import (
	"github.com/DaiYuANg/warden/server/internal/config"
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
	return humafiber.New(app, huma.DefaultConfig("warden", "0.1"))
}
