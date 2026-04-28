package httpserver

import (
	"log/slog"

	authhttp "github.com/arcgolabs/authx/http"
	authfiber "github.com/arcgolabs/authx/http/fiber"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/config"
)

// newFiberAppAndRuntime wires Fiber + httpx with OpenAPI defaults and optional deploy-route auth.
func newFiberAppAndRuntime(cfg config.Config, logger *slog.Logger, guard *authhttp.Guard) (*fiber.App, httpx.ServerRuntime) {
	fiberApp := fiber.New()
	fiberAdapter := adapterfiber.New(fiberApp, adapter.HumaOptions{
		Title:       "orch API",
		Version:     buildmeta.Version(),
		Description: "Orch control plane API",
		DocsPath:    OpenAPIDocsPath,
		OpenAPIPath: OpenAPIJSONPath,
	})
	if guard != nil {
		fiberApp.Use("/api/v1/deploy", authfiber.Require(guard))
	}

	rt := httpx.New(
		httpx.WithAdapter(fiberAdapter),
		httpx.WithLogger(logger),
		httpx.WithValidation(),
		httpx.WithBasePath("/api"),
	)
	return fiberApp, rt
}
