package orchvpn

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
)

// NewWorkstationApp builds the short-lived dix graph for the orch-vpn workstation daemon.
// cfg supplies log and other ORCH_* settings (same loader as orch-server). Call Start, then resolve
// [*WorkstationDaemon] and Run(ctx); OnStop closes the HTTP client.
func NewWorkstationApp(conn WorkstationConn, cfg config.Config) *dix.App {
	return dix.New(
		"orch-vpn-workstation",
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom1(func(logger *slog.Logger) *slog.Logger { return logger }),
		dix.WithModules(
			config.Static(cfg),
			logging.Module(),
			workstationModule(conn),
		),
	)
}

func workstationModule(conn WorkstationConn) dix.Module {
	return dix.NewModule(
		"orch-vpn-ws",
		dix.Providers(
			dix.Provider0(func() WorkstationConn { return conn }),
			dix.ProviderErr1(func(c WorkstationConn) (ClientConfig, error) {
				return c.ClientConfig()
			}),
			dix.ProviderErr1(func(cfg ClientConfig) (*apiclient.Client, error) {
				return apiclient.New(cfg.ControlPlaneURL, cfg.BearerToken)
			}),
			dix.Provider3(NewWorkstationDaemon),
		),
		dix.Hooks(
			dix.OnStop(func(_ context.Context, c *apiclient.Client) error {
				if c == nil {
					return nil
				}
				return c.Close()
			}),
		),
	)
}
