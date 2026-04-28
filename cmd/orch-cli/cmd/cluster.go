package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newHealthCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check connectivity to the orch control plane",
		Long:  `Contacts the server configured with --server (or ORCH_SERVER) and reports readiness.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client) error {
				out, err := c.Health(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "health")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				return fprintfStdout("status=%s time=%s\n", out.Body.Status, out.Body.Timestamp)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newHostinfoCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "hostinfo",
		Short: "Show CPU, memory, load, and disk snapshot from the server host",
		Long:  `Diagnostics for the node your --server resolves to (or any peer in a cluster when pointed at it).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client) error {
				out, err := c.Hostinfo(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "hostinfo")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeHostinfoHuman(out)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON (full hostinfo report)")
	return cmd
}

func newWorkloadsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "workloads",
		Short: "List workloads registered on the server",
		Long:  `Shows workloads this control plane node knows about for the current cluster context (--server).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client) error {
				out, err := c.ListWorkloads(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "list workloads")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body.Items)
				}
				return writeWorkloadsHuman(out.Body.Items)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newApplyCmd() *cobra.Command {
	var file string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Deploy a manifest to the cluster from a YAML file",
		Long: `Loads and validates the deploy YAML locally, then submits it to orch-server so workloads and
related resources can be reconciled. Requires a reachable control plane (--server / ORCH_SERVER).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return oopsx.B("cli").Errorf("--file is required")
			}
			app, err := deployv1.LoadAppFile(file)
			if err != nil {
				return oopsx.B("cli").Wrapf(err, "load manifest")
			}
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client) error {
				out, err := c.Deploy(ctx, app)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "deploy")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				return fprintfStdout("accepted app=%s workloads=%d\n", out.Body.App, out.Body.Workloads)
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON response")
	return cmd
}

func writeHostinfoHuman(out *api.HostinfoOutput) error {
	body := out.Body
	h := body.Host
	cpu := body.CPU
	mem := body.Memory
	if err := fprintfStdout("hostname=%s os=%s/%s kernel=%s arch=%s\n",
		h.Hostname, h.OS, h.Platform, h.KernelVersion, h.KernelArch); err != nil {
		return err
	}
	if err := fprintfStdout("cpu_cores=%d model=%s usage_percent=%.1f\n",
		cpu.LogicalCores, cpu.ModelName, cpu.UsagePercent); err != nil {
		return err
	}
	if err := fprintfStdout("memory_total_bytes=%d used_percent=%.1f\n",
		mem.TotalBytes, mem.UsedPercent); err != nil {
		return err
	}
	if body.Load != nil {
		l := body.Load
		return fprintfStdout("load_1=%.2f load_5=%.2f load_15=%.2f\n", l.Load1, l.Load5, l.Load15)
	}
	return nil
}

func writeWorkloadsHuman(items []registry.WorkloadRecord) error {
	if err := fprintfStdout("NAME\tNODE\tRUNTIME\tSTATUS\tIMAGE\n"); err != nil {
		return err
	}
	for _, w := range items {
		node := w.Node
		if node == "" {
			node = "-"
		}
		if err := fprintfStdout("%s\t%s\t%s\t%s\t%s\n", w.Name, node, w.Runtime, w.Status, w.Image); err != nil {
			return err
		}
	}
	return nil
}

func fprintfStdout(format string, a ...any) error {
	_, err := fmt.Fprintf(os.Stdout, format, a...)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "write stdout")
	}
	return nil
}

func contextFromCmd(cmd *cobra.Command) context.Context {
	if cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}
