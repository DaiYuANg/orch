package orchvpn

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
)

// GatewayModule runs the UDP tunnel listener on the orchestrator when orch_vpn.enabled is true.
func GatewayModule() dix.Module {
	return dix.NewModule(
		"orchvpn-gateway",
		dix.Providers(
			dix.Provider1(func(cfg config.Config) config.OrchVPNConfig {
				return cfg.OrchVPN
			}),
			dix.Provider2(NewGatewayService),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, g *GatewayService) error {
				logger.Info("lifecycle", "phase", "starting", "component", "orchvpn-gateway")
				if err := g.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "orchvpn-gateway", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "orchvpn-gateway")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, g *GatewayService) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "orchvpn-gateway")
				if err := g.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "orchvpn-gateway", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "orchvpn-gateway")
				return nil
			}),
		),
	)
}
