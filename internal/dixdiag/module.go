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
		),
		dix.Setups(
			dix.SetupWithMetadata(func(c *dix.Container, _ dix.Lifecycle) error {
				cfg, err := dix.ResolveAs[config.Config](c)
				if err != nil {
					return err
				}
				raft, err := dix.ResolveAs[*raftsvc.Service](c)
				if err != nil {
					return err
				}
				svc, err := dix.ResolveAs[*Service](c)
				if err != nil {
					return err
				}
				registerHealthChecks(c, cfg, raft, svc)
				return nil
			}, dix.SetupMetadata{
				Label: "register dix diagnostics",
				Dependencies: dix.ServiceRefs(
					dix.TypedService[config.Config](),
					dix.TypedService[*raftsvc.Service](),
					dix.TypedService[*Service](),
				),
			}),
		),
	)
}

func registerHealthChecks(c *dix.Container, cfg config.Config, raft *raftsvc.Service, svc *Service) {
	if c == nil {
		return
	}
	c.RegisterHealthCheck("runtime", func(ctx context.Context) error {
		if svc == nil || svc.Runtime() == nil {
			return ErrRuntimeUnavailable
		}
		return nil
	})
	c.RegisterLivenessCheck("runtime", func(ctx context.Context) error {
		if svc == nil || svc.Runtime() == nil {
			return ErrRuntimeUnavailable
		}
		return nil
	})
	c.RegisterReadinessCheck("control-plane", func(ctx context.Context) error {
		return CheckControlPlaneReady(ctx, cfg, raft)
	})
}
