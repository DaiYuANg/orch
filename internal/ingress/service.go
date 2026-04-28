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
		Logging: caddyOrchSlogLogging(),
		AppsRaw: httpApps.All(),
	}
	setCaddySlogBridge(s.logger.With(slog.String("component", "ingress"), slog.String("engine", "caddy")))
	defer clearCaddySlogBridge()

	if err := caddy.Run(cfg); err != nil {
		return err
	}
	s.started.Store(true)
	s.logger.Info("ingress started with embedded caddy", "listen", listen)
	return nil
}

// caddyBaseOrchLog is shared by the default log and the stdlib "sink" so zap + log.Printf both hit the same orch slog bridge.
func caddyBaseOrchLog() caddy.BaseLog {
	return caddy.BaseLog{
		WriterRaw:  json.RawMessage(`{"output":"orch_slog"}`),
		EncoderRaw: json.RawMessage(`{"format":"json"}`),
		Level:      "INFO",
	}
}

// caddyOrchSlogLogging sends Caddy/zap (including RedirectStdLog) through the orch slog bridge.
func caddyOrchSlogLogging() *caddy.Logging {
	bl := caddyBaseOrchLog()
	logs := mapping.NewMap[string, *caddy.CustomLog]()
	logs.Set(caddy.DefaultLoggerName, &caddy.CustomLog{BaseLog: bl})
	return &caddy.Logging{
		Sink: &caddy.SinkLog{BaseLog: bl},
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
