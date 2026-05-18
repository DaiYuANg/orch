package dixdiag

import (
	"context"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
)

func Module() dix.Module {
	return dix.NewModule(
		"dixdiag",
		dix.Providers(
			dix.Provider0(New, dix.Eager()),
			dix.Provider3(newHealthCheckDeps),
		),
		dix.Setups(
			dix.SetupWithMetadata(func(c *dix.Container, _ dix.Lifecycle) error {
				deps, err := dix.ResolveAs[healthCheckDeps](c)
				if err != nil {
					return err
				}
				registerHealthChecks(c, deps)
				return nil
			}, dix.SetupMetadata{
				Label: "register dix diagnostics",
				Dependencies: dix.ServiceRefs(
					dix.TypedService[healthCheckDeps](),
				),
			}),
		),
	)
}

type healthCheckDeps struct {
	Config config.Config
	Raft   *raftsvc.Service
	Svc    *Service
}

func newHealthCheckDeps(cfg config.Config, raft *raftsvc.Service, svc *Service) healthCheckDeps {
	return healthCheckDeps{Config: cfg, Raft: raft, Svc: svc}
}

func registerHealthChecks(c *dix.Container, deps healthCheckDeps) {
	if c == nil {
		return
	}
	c.RegisterHealthCheck("runtime", func(ctx context.Context) error {
		if deps.Svc == nil || deps.Svc.Runtime() == nil {
			return ErrRuntimeUnavailable
		}
		return nil
	})
	c.RegisterLivenessCheck("runtime", func(ctx context.Context) error {
		if deps.Svc == nil || deps.Svc.Runtime() == nil {
			return ErrRuntimeUnavailable
		}
		return nil
	})
	c.RegisterReadinessCheck("control-plane", func(ctx context.Context) error {
		return CheckControlPlaneReady(ctx, deps.Config, deps.Raft)
	})
}
