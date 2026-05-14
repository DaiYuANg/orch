package orchvpn

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
)

// GatewayModule runs the UDP tunnel listener on the orchestrator when orch_vpn.enabled is true.
func GatewayModule() dix.Module {
	return dix.NewModule(
		"orchvpn-gateway",
		dix.Providers(
			dix.Provider1(func(cfg config.Config) config.OrchVPNConfig {
				return cfg.OrchVPN
			}, dix.Eager()),
			dix.Provider2(NewGatewayService, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, g *GatewayService) error {
				logger.Info("lifecycle", "phase", "starting", "component", "orchvpn-gateway")
				if err := g.Start(context.WithoutCancel(ctx)); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "orchvpn-gateway", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "orchvpn-gateway")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookOrchVPN), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStart)),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, g *GatewayService) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "orchvpn-gateway")
				if err := g.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "orchvpn-gateway", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "orchvpn-gateway")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookOrchVPN), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStop)),
		),
	)
}
