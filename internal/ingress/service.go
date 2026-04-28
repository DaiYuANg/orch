package ingress

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/standard"

	"github.com/arcgolabs/collectionx/mapping"

	"github.com/daiyuang/orch/internal/config"
)

type Service struct {
	logger  *slog.Logger
	cfg     config.IngressConfig
	started atomic.Bool
}

func New(cfg config.Config, logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
		cfg:    cfg.Ingress,
	}
}

func (s *Service) Start(_ context.Context) error {
	if !s.cfg.Enabled {
		s.logger.Info("ingress disabled by config")
		return nil
	}
	if s.started.Load() {
		return nil
	}

	handler := caddyconfig.JSONModuleObject(
		caddyhttp.StaticResponse{
			StatusCode: "200",
			Body:       "orch ingress is running",
		},
		"handler",
		"static_response",
		nil,
	)

	listen := s.cfg.ListenAddrs()
	servers := mapping.NewMap[string, *caddyhttp.Server]()
	servers.Set("orch-ingress", &caddyhttp.Server{
		Listen: listen,
		// Non-nil enables HTTP access logs ("handled request") on the default logger.
		Logs: &caddyhttp.ServerLogConfig{},
		Routes: caddyhttp.RouteList{
			{
				HandlersRaw: []json.RawMessage{handler},
			},
		},
	})
	app := caddyhttp.App{
		Servers: servers.All(),
	}

	httpApps := mapping.NewMap[string, json.RawMessage]()
	httpApps.Set("http", caddyconfig.JSON(app, nil))

	cfg := &caddy.Config{
		Admin: &caddy.AdminConfig{
			Disabled: true,
		},
		// Align with orch slog/logx (stdout). Caddy defaults to stderr, so logs were easy to miss.
		Logging: caddyStdoutLogging(),
		AppsRaw: httpApps.All(),
	}
	if err := caddy.Run(cfg); err != nil {
		return err
	}
	s.started.Store(true)
	s.logger.Info("ingress started with embedded caddy", "listen", listen)
	return nil
}

// caddyStdoutLogging wires Caddy's default zap logger to stdout like logx-backed slog.
func caddyStdoutLogging() *caddy.Logging {
	logs := mapping.NewMap[string, *caddy.CustomLog]()
	logs.Set(caddy.DefaultLoggerName, &caddy.CustomLog{
		BaseLog: caddy.BaseLog{
			WriterRaw: json.RawMessage(`{"output":"stdout"}`),
			Level:     "INFO",
		},
	})
	return &caddy.Logging{
		Logs: logs.All(),
	}
}

func (s *Service) Stop(_ context.Context) error {
	if !s.started.Load() {
		return nil
	}
	if err := caddy.Stop(); err != nil {
		return err
	}
	s.started.Store(false)
	s.logger.Info("ingress stopped")
	return nil
}
