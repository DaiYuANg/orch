package http

import (
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
			EnablePrintRoutes: true,
		},
	)

	return app
}

func newOpenapi(app *fiber.App) huma.API {
	return humafiber.New(app, huma.DefaultConfig("warden", "0.1"))
}
