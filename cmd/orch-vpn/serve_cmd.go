package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/orchvpn"
)

func newServeCmd() *cobra.Command {
	var listenUDP string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a standalone orch-vpn UDP encap-v0 listener (dev; mirrors gateway frame handling)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			appCfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			svcCfg := orchvpn.ServerConfig{ListenUDP: listenUDP}
			app := orchvpn.NewServeApp(svcCfg, appCfg)
			rt, err := app.Start(cmd.Context())
			if err != nil {
				return fmt.Errorf("start orch-vpn serve: %w", err)
			}
			stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(cmd.Context()), 10*time.Second)
			defer cancelStop()
			defer func() {
				if stopErr := rt.Stop(stopCtx); stopErr != nil {
					pterm.Warning.Printfln("orch-vpn serve stop: %v", stopErr)
				}
			}()

			svc, err := dix.ResolveAs[*orchvpn.ServerDaemonService](rt.Container())
			if err != nil {
				return fmt.Errorf("resolve serve daemon: %w", err)
			}
			if err := svc.Run(cmd.Context()); err != nil {
				return fmt.Errorf("orch-vpn serve: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&listenUDP, "listen-udp", ":15888", "UDP listen address for encap-v0")
	return cmd
}
