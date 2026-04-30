package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/orchvpn"
)

func newRootCmd() *cobra.Command {
	var (
		serverURL string
		token     string
		periodSec int
		enableTUN bool
		tunName   string
	)

	runDaemon := func(cmd *cobra.Command, args []string) error {
		_ = args
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		conn := orchvpn.WorkstationConn{
			ServerURL:       strings.TrimSpace(serverURL),
			Token:           strings.TrimSpace(token),
			HealthPeriodSec: periodSec,
			EnableTUN:       enableTUN,
			TUNName:         strings.TrimSpace(tunName),
		}
		app := orchvpn.NewWorkstationApp(conn, cfg)
		rt, err := app.Start(cmd.Context())
		if err != nil {
			return fmt.Errorf("start orch-vpn workstation: %w", err)
		}
		stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(cmd.Context()), 10*time.Second)
		defer cancelStop()
		defer func() {
			if stopErr := rt.Stop(stopCtx); stopErr != nil {
				pterm.Warning.Printfln("orch-vpn workstation stop: %v", stopErr)
			}
		}()

		d, err := dix.ResolveAs[*orchvpn.WorkstationDaemon](rt.Container())
		if err != nil {
			return fmt.Errorf("resolve workstation daemon: %w", err)
		}
		return d.Run(cmd.Context())
	}

	root := &cobra.Command{
		Use:           "orch-vpn",
		Short:         "orch-vpn — connect your machine to orchestrator container networks (TUN + tunnel)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildmeta.Version(),
		Long: `orch-vpn runs as a background daemon on a developer or operator workstation.
It will negotiate an encrypted tunnel with the orch cluster so traffic can reach container
private addresses. The data plane (TUN + encapsulation) is under development; today this
process only verifies control-plane connectivity on an interval. Use --tun to create a TUN and forward IPv4
inside encap-v0 to the cluster gateway (needs administrator / elevated privileges on Windows, root on Unix).

Cobra-level messages (e.g. startup errors, dix Stop warnings) use pterm; the workstation/serve daemons log via the injected slog instance (configure ORCH_* / internal/logging).`,
		RunE: runDaemon,
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.PersistentFlags().StringVarP(&serverURL, "server", "s", "",
		`orch control plane base URL (default: env ORCH_SERVER or http://127.0.0.1:17443)`)
	root.PersistentFlags().StringVar(&token, "token", "", "Bearer token when auth is enabled (env ORCH_TOKEN)")
	root.PersistentFlags().IntVar(&periodSec, "health-period", 60, "seconds between control-plane health probes")
	root.PersistentFlags().BoolVar(&enableTUN, "tun", false, "Create a TUN and forward encap-v0 IPv4 (admin / CAP_NET_ADMIN)")
	root.PersistentFlags().StringVar(&tunName, "tun-name", "", "TUN interface name (OS default if empty: orch-vpn / utun / orch-vpn0)")

	clientAlias := &cobra.Command{
		Use:   "client",
		Short: "Same as the default daemon (workstation side)",
		RunE:  runDaemon,
	}
	root.AddCommand(clientAlias)
	root.AddCommand(newServeCmd())
	return root
}
