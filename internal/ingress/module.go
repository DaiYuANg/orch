package ingress

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
)

// Module wires ingress: arcgolabs/vale reverse proxy/LB and optional Let's Encrypt autocert on HTTPS listeners.
// *ingress.Service for lifecycle and DI. Data-plane path routes are compiled from desired deploy apps
// (ingresses) and workload DNS registrations, not from static config.
//
// Register this module after raft and dns so start order remains: raft → dns → ingress → …
func Module() dix.Module {
	return dix.NewModule(
		"ingress",
		dix.Providers(
			dix.Provider4(New),
		),
		dix.Invokes(
			dix.Invoke3(func(logger *slog.Logger, cfg config.Config, _ *Service) {
				if !cfg.Ingress.Enabled {
					return
				}
				log := logger.With(slog.String("component", "ingress"))
				log.Debug("ingress di: *ingress.Service registered for injection")
				log.Info("ingress routes source: deploy documents (ingresses); listeners from ingress.listen / ingress.tls")
				if cfg.Ingress.TLS.Enabled {
					log.Info("ingress tls autocert",
						slog.Bool("enabled", true),
						slog.Any("domains", cfg.Ingress.TLSAutocertDomains()),
						slog.Any("tls_listen", cfg.Ingress.TLSListenAddrs()),
						slog.Bool("staging", cfg.Ingress.TLS.Staging),
					)
				}
			}),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "ingress")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "ingress", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "ingress")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "ingress")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "ingress", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "ingress")
				return nil
			}),
		),
	)
}
