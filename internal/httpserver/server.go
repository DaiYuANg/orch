package httpserver

import (
	"context"
	"log/slog"

	authhttp "github.com/arcgolabs/authx/http"
	authfiber "github.com/arcgolabs/authx/http/fiber"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/observability"
)

type Server struct {
	logger  *slog.Logger
	addr    string
	runtime httpx.ServerRuntime
}

func New(cfg config.Config, logger *slog.Logger, guard *authhttp.Guard, obs *observability.Service) (*Server, error) {
	fiberApp := fiber.New()
	fiberAdapter := adapterfiber.New(fiberApp, adapter.HumaOptions{
		Title:       "orch API",
		Version:     "v0.1.0",
		Description: "Orch control plane API",
		DocsPath:    "/swagger-ui",
		OpenAPIPath: "/openapi.json",
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
	attachFiberPrometheus(fiberApp, cfg, obs)

	return &Server{
		logger:  logger,
		addr:    cfg.HTTP.Addr,
		runtime: rt,
	}, nil
}

func (s *Server) Runtime() httpx.ServerRuntime {
	return s.runtime
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("http server starting", "addr", s.addr)
	go func() {
		if err := s.runtime.ListenAndServeContext(ctx, s.addr); err != nil {
			s.logger.Error("http server stopped with error", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(_ context.Context) error {
	s.logger.Info("http server stopping")
	return s.runtime.Shutdown()
}
