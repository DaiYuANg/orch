package http

import (
	"github.com/DaiYuANg/toolkit4go/httpx/adapter"
	httpxfiber "github.com/DaiYuANg/toolkit4go/httpx/adapter/fiber"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("http",
	fx.Provide(
		newFiberAdapter,
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

func newFiberAdapter() *httpxfiber.Adapter {
	app := fiber.New(
		fiber.Config{
			DisableStartupMessage: true,
			ReduceMemoryUsage:     true,
			EnablePrintRoutes:     false,
		},
	)
	humaOpts := adapter.HumaOptions{
		Enabled:     true,
		Title:       "warden",
		Version:     "0.1",
		Description: "warden api",
		DocsPath:    "/",
		OpenAPIPath: "/openapi.json",
	}
	return httpxfiber.New(app).WithHuma(humaOpts)
}

func newFiber(fiberAdapter *httpxfiber.Adapter) *fiber.App {
	return fiberAdapter.App()
}

func newOpenapi(fiberAdapter *httpxfiber.Adapter) huma.API {
	return fiberAdapter.HumaAPI()
}
