package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/workloadmeta"
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
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.Health(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "health")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				pterm.Info.Printfln("status=%s time=%s", out.Body.Status, out.Body.Timestamp)
				return nil
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
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
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
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
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

func newAssignmentsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "assignments",
		Short: "List scheduler workload assignments",
		Long:  `Shows persisted scheduler decisions and deploy results from the current control plane context (--server).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.ListAssignments(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "list assignments")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body.Items)
				}
				return writeAssignmentsHuman(out.Body.Items)
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
		Short: "Deploy a manifest to the cluster from a .orch or YAML file",
		Long: `Reads the deploy file locally and sends its source to orch-server. The server parses the document (virtual path
suffix selects .orch vs YAML), replicates desired state through Raft, then reconciles workloads on each node.
Requires a reachable control plane (--server / ORCH_SERVER); clustered Raft deploys must target the leader.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			if file == "" {
				return oopsx.B("cli").Errorf("--file is required")
			}
			src, err := os.ReadFile(file)
			if err != nil {
				return oopsx.B("cli").Wrapf(err, "read manifest file")
			}
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.DeploySource(ctx, filepath.Base(file), string(src))
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "deploy")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				pterm.Success.Printfln("accepted app=%s workloads=%d", out.Body.App, out.Body.Workloads)
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy file (.orch or YAML)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON response")
	return cmd
}

func writeHostinfoHuman(out *api.HostinfoOutput) error {
	body := out.Body
	h := body.Host
	cpu := body.CPU
	mem := body.Memory
	rows := pterm.TableData{
		{"Property", "Value"},
		{"hostname", h.Hostname},
		{"os", h.OS},
		{"platform", h.Platform},
		{"kernel", h.KernelVersion},
		{"arch", h.KernelArch},
		{"cpu_cores", strconv.Itoa(cpu.LogicalCores)},
		{"cpu_model", cpu.ModelName},
		{"cpu_usage_pct", strconv.FormatFloat(cpu.UsagePercent, 'f', 1, 64)},
		{"memory_total_bytes", strconv.FormatUint(mem.TotalBytes, 10)},
		{"memory_used_pct", strconv.FormatFloat(mem.UsedPercent, 'f', 1, 64)},
	}
	if body.Load != nil {
		l := body.Load
		rows = append(rows, []string{"load_1", strconv.FormatFloat(l.Load1, 'f', 2, 64)})
		rows = append(rows, []string{"load_5", strconv.FormatFloat(l.Load5, 'f', 2, 64)})
		rows = append(rows, []string{"load_15", strconv.FormatFloat(l.Load15, 'f', 2, 64)})
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(rows).Render(); err != nil {
		return oopsx.B("cli").Wrapf(err, "render hostinfo table")
	}
	return nil
}

func writeWorkloadsHuman(items []registry.WorkloadRecord) error {
	rows := pterm.TableData{{"NAME", "NODE", "RUNTIME", "STATUS", "IMAGE"}}
	for _, w := range items {
		node := w.Node
		if node == "" {
			node = "-"
		}
		rows = append(rows, []string{w.Name, node, w.Runtime, w.Status, w.Image})
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(rows).Render(); err != nil {
		return oopsx.B("cli").Wrapf(err, "render workloads table")
	}
	return nil
}

func writeAssignmentsHuman(items []workloadmeta.Assignment) error {
	rows := pterm.TableData{{"KEY", "NODE", "RUNTIME", "STATUS", "IMAGE", "ERROR"}}
	for _, a := range items {
		node := nonEmpty(a.Node)
		image := nonEmpty(a.Image)
		errMsg := nonEmpty(a.Error)
		rows = append(rows, []string{a.Key, node, string(a.Runtime), a.Status, image, errMsg})
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(rows).Render(); err != nil {
		return oopsx.B("cli").Wrapf(err, "render assignments table")
	}
	return nil
}

func nonEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func contextFromCmd(cmd *cobra.Command) context.Context {
	if cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}
