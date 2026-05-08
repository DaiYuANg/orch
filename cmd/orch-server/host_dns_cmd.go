package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/hostdns"
)

func newHostDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host-dns",
		Short: "Manage host OS resolver integration for orch DNS",
		Long:  "Installer-facing commands that configure the host OS resolver to route the orch DNS zone to orch-server.",
	}
	cmd.PersistentFlags().String("config", "", "Path to YAML, JSON, or TOML config file")
	config.BindOrchFlags(cmd.PersistentFlags(), config.Default())

	cmd.AddCommand(newHostDNSActionCmd("install"))
	cmd.AddCommand(newHostDNSActionCmd("uninstall"))
	cmd.AddCommand(newHostDNSStatusCmd())
	return cmd
}

func newHostDNSActionCmd(action string) *cobra.Command {
	var jsonOut bool
	var nonInteractive bool
	cmd := &cobra.Command{
		Use:   action,
		Short: action + " host DNS resolver integration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = nonInteractive // reserved for installer parity; commands are non-interactive today.
			cfg, err := config.LoadFromCobra(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			hostCfg, err := hostdns.ConfigFromOrch(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			manager := hostdns.DefaultManager()
			switch action {
			case "install":
				err = manager.Install(ctx, hostCfg)
			case "uninstall":
				err = manager.Uninstall(ctx, hostCfg)
			default:
				err = fmt.Errorf("unknown host-dns action %q", action)
			}
			if err != nil {
				return err
			}
			if jsonOut {
				return writeHostDNSJSON(map[string]any{
					"action": action,
					"config": hostCfg,
					"ok":     true,
				})
			}
			fmt.Fprintf(os.Stdout, "host-dns %s ok: zone=%s nameserver=%s port=%d\n", action, hostCfg.Zone, hostCfg.Nameserver, hostCfg.Port)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Run without prompts for installer scripts")
	return cmd
}

func newHostDNSStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show host DNS resolver integration status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromCobra(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			hostCfg, err := hostdns.ConfigFromOrch(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			status, err := hostdns.DefaultManager().Status(ctx, hostCfg)
			if err != nil {
				return err
			}
			if jsonOut {
				return writeHostDNSJSON(status)
			}
			fmt.Fprintf(os.Stdout, "host-dns supported=%t installed=%t zone=%s nameserver=%s port=%d detail=%s\n",
				status.Supported,
				status.Installed,
				status.Config.Zone,
				status.Config.Nameserver,
				status.Config.Port,
				status.Detail,
			)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func writeHostDNSJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
