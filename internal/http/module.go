package http

import (
	"github.com/DaiYuANg/toolkit4go/httpx/adapter"
	httpxfiber "github.com/DaiYuANg/toolkit4go/httpx/adapter/fiber"
	"github.com/DaiYuANg/warden/internal/config"
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

func newFiberAdapter(cfg *config.Config) *httpxfiber.Adapter {
	app := fiber.New(
		fiber.Config{
			DisableStartupMessage: cfg.Http.DisableStartupMessage,
			ReduceMemoryUsage:     cfg.Http.ReduceMemoryUsage,
			EnablePrintRoutes:     cfg.Http.PrintRoutes,
		},
	)
	apiDocCfg := cfg.Http.APIDoc
	humaOpts := adapter.HumaOptions{
		Enabled:     apiDocCfg.Enable,
		Title:       apiDocCfg.Title,
		Version:     apiDocCfg.Version,
		Description: apiDocCfg.Description,
		DocsPath:    apiDocCfg.Path,
		OpenAPIPath: apiDocCfg.OpenAPIPath,
	}
	return httpxfiber.New(app).WithHuma(humaOpts)
}

func newFiber(fiberAdapter *httpxfiber.Adapter) *fiber.App {
	return fiberAdapter.App()
}

func newOpenapi(fiberAdapter *httpxfiber.Adapter) huma.API {
	return fiberAdapter.HumaAPI()
}
