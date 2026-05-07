package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/deploy/loader"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
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
				return writeInfoLine("health",
					viewField("status", statusBadge(out.Body.Status)),
					viewField("time", out.Body.Timestamp),
				)
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
	return newWorkloadsListCmd("workloads", []string{"workload", "ps"})
}

func newWorkloadsListCmd(use string, aliases []string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   "List workloads registered on the server",
		Long:    `Shows workloads this control plane node knows about for the current cluster context (--server).`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			return runListWorkloads(ctx, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newAssignmentsCmd() *cobra.Command {
	return newAssignmentsListCmd("assignments", []string{"assignment"})
}

func newAssignmentsListCmd(use string, aliases []string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   "List scheduler workload assignments",
		Long:    `Shows persisted scheduler decisions and deploy results from the current control plane context (--server).`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			return runListAssignments(ctx, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display cluster resources",
		Long:  `Display cluster resources from the current control plane context (--server), similar to kubectl get.`,
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newWorkloadsListCmd("workloads", []string{"workload", "wl", "ps"}))
	cmd.AddCommand(newAssignmentsListCmd("assignments", []string{"assignment", "assign"}))
	return cmd
}

func newApplyCmd() *cobra.Command {
	var file string
	var jsonOut bool
	var watch bool
	var timeout time.Duration
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
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, deploy *loader.Loader) error {
				var app *deployv1.App
				if watch {
					var loadErr error
					app, loadErr = deploy.LoadAppString(ctx, filepath.Base(file), string(src))
					if loadErr != nil {
						return oopsx.B("cli").Wrapf(loadErr, "parse manifest for watch")
					}
				}
				out, err := c.DeploySource(ctx, filepath.Base(file), string(src))
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "deploy")
				}
				if jsonOut && !watch {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				if !jsonOut {
					if err := writeInfoLine("deploy",
						viewField("status", statusBadge("accepted")),
						viewField("app", out.Body.App),
						viewField("workloads", strconv.Itoa(out.Body.Workloads)),
					); err != nil {
						return err
					}
				}
				if !watch {
					return nil
				}
				snapshot, err := watchDeployment(ctx, c, app, timeout, !jsonOut)
				if err != nil {
					return err
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(applyWatchOutput{
						Accepted:    out.Body.Accepted,
						App:         out.Body.App,
						Workloads:   out.Body.Workloads,
						Assignments: snapshot.Assignments,
						Registry:    snapshot.Workloads,
					})
				}
				if err := writeAssignmentsHuman(snapshot.Assignments); err != nil {
					return err
				}
				if err := writeWorkloadsHuman(snapshot.Workloads); err != nil {
					return err
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy file (.orch or YAML)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON response")
	cmd.Flags().BoolVar(&watch, "watch", false, "Wait until deployed workloads are running")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Maximum time to wait with --watch")
	return cmd
}

func runListWorkloads(ctx context.Context, jsonOut bool) error {
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
}

func runListAssignments(ctx context.Context, jsonOut bool) error {
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
}

type applyWatchOutput struct {
	Accepted    bool                 `json:"accepted"`
	App         string               `json:"app"`
	Workloads   int                  `json:"workloads"`
	Assignments []api.AssignmentItem `json:"assignments"`
	Registry    []api.WorkloadItem   `json:"registry"`
}

type deploySnapshot struct {
	Assignments        []api.AssignmentItem
	Workloads          []api.WorkloadItem
	Total              int
	RunningAssignments int
	RunningWorkloads   int
	FailedAssignment   *api.AssignmentItem
}

func watchDeployment(ctx context.Context, c *apiclient.Client, app *deployv1.App, timeout time.Duration, progress bool) (*deploySnapshot, error) {
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	expectedKeys, expectedNames := expectedDeployWorkloads(app)
	if len(expectedKeys) == 0 {
		return &deploySnapshot{}, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	spinner := startWatchSpinner(progress, len(expectedKeys))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var last *deploySnapshot
	var lastErr error
	for {
		snapshot, err := readDeploySnapshot(waitCtx, c, expectedKeys, expectedNames)
		if err != nil {
			lastErr = err
			updateWatchSpinner(spinner, last, len(expectedKeys), err)
		} else {
			last = snapshot
			updateWatchSpinner(spinner, snapshot, len(expectedKeys), nil)
			switch {
			case snapshot.FailedAssignment != nil:
				failWatchSpinner(spinner, fmt.Sprintf("workload failed key=%s", snapshot.FailedAssignment.Key))
				return snapshot, oopsx.B("cli").Errorf("workload %s failed on node %s: %s",
					snapshot.FailedAssignment.Key,
					nonEmpty(snapshot.FailedAssignment.Node),
					nonEmpty(snapshot.FailedAssignment.Error),
				)
			case snapshot.RunningAssignments == snapshot.Total && snapshot.RunningWorkloads == snapshot.Total:
				successWatchSpinner(spinner, fmt.Sprintf("workloads running assignments=%d/%d runtime=%d/%d",
					snapshot.RunningAssignments, snapshot.Total, snapshot.RunningWorkloads, snapshot.Total))
				return snapshot, nil
			}
		}

		select {
		case <-waitCtx.Done():
			failWatchSpinner(spinner, watchStatusText(last, len(expectedKeys), "timed out"))
			if lastErr != nil {
				return last, oopsx.B("cli").Wrapf(lastErr, "wait for deploy status timed out after %s", timeout)
			}
			return last, oopsx.B("cli").Errorf("wait for deploy status timed out after %s: %s", timeout, watchStatusText(last, len(expectedKeys), ""))
		case <-ticker.C:
		}
	}
}

func expectedDeployWorkloads(app *deployv1.App) (map[string]struct{}, map[string]struct{}) {
	keys := make(map[string]struct{})
	names := make(map[string]struct{})
	if app == nil {
		return keys, names
	}
	for _, workload := range app.Workloads {
		key := workloadmeta.AssignmentKey(app.Metadata, workload.Name)
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
		names[workload.Name] = struct{}{}
	}
	return keys, names
}

func readDeploySnapshot(ctx context.Context, c *apiclient.Client, expectedKeys, expectedNames map[string]struct{}) (*deploySnapshot, error) {
	assignments, err := c.ListAssignments(ctx)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "list assignments")
	}
	workloads, err := c.ListWorkloads(ctx)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "list workloads")
	}

	snapshot := &deploySnapshot{Total: len(expectedKeys)}
	for _, assignment := range assignments.Body.Items {
		if _, ok := expectedKeys[assignment.Key]; !ok {
			continue
		}
		snapshot.Assignments = append(snapshot.Assignments, assignment)
		if assignment.Status == workloadmeta.AssignmentStatusRunning {
			snapshot.RunningAssignments++
		}
		if assignment.Status == workloadmeta.AssignmentStatusFailed && snapshot.FailedAssignment == nil {
			failed := assignment
			snapshot.FailedAssignment = &failed
		}
	}
	for _, workload := range workloads.Body.Items {
		if _, ok := expectedNames[workload.Name]; !ok {
			continue
		}
		snapshot.Workloads = append(snapshot.Workloads, workload)
		if workload.Status == "running" {
			snapshot.RunningWorkloads++
		}
	}
	return snapshot, nil
}

func startWatchSpinner(progress bool, total int) *pterm.SpinnerPrinter {
	if !progress || !stderrIsTerminal() {
		return nil
	}
	spinner, err := pterm.DefaultSpinner.WithRemoveWhenDone(false).Start(
		fmt.Sprintf("waiting for workloads assignments=0/%d runtime=0/%d", total, total),
	)
	if err != nil {
		return nil
	}
	return spinner
}

func updateWatchSpinner(spinner *pterm.SpinnerPrinter, snapshot *deploySnapshot, total int, err error) {
	if spinner == nil {
		return
	}
	if err != nil {
		spinner.UpdateText(watchStatusText(snapshot, total, fmt.Sprintf("last_error=%v", err)))
		return
	}
	spinner.UpdateText(watchStatusText(snapshot, total, ""))
}

func successWatchSpinner(spinner *pterm.SpinnerPrinter, msg string) {
	if spinner != nil {
		spinner.Success(msg)
	}
}

func failWatchSpinner(spinner *pterm.SpinnerPrinter, msg string) {
	if spinner != nil {
		spinner.Fail(msg)
	}
}

func watchStatusText(snapshot *deploySnapshot, total int, suffix string) string {
	runningAssignments := 0
	runningWorkloads := 0
	if snapshot != nil {
		runningAssignments = snapshot.RunningAssignments
		runningWorkloads = snapshot.RunningWorkloads
		if snapshot.Total > 0 {
			total = snapshot.Total
		}
	}
	text := fmt.Sprintf("waiting for workloads assignments=%d/%d runtime=%d/%d", runningAssignments, total, runningWorkloads, total)
	if suffix != "" {
		return text + " " + suffix
	}
	return text
}

func stderrIsTerminal() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func writeHostinfoHuman(out *api.HostinfoOutput) error {
	body := out.Body
	h := body.Host
	cpu := body.CPU
	mem := body.Memory
	rows := [][]string{
		{"hostname", h.Hostname},
		{"os", h.OS},
		{"platform", h.Platform},
		{"kernel", h.KernelVersion},
		{"arch", h.KernelArch},
		{"cpu_cores", strconv.Itoa(cpu.LogicalCores)},
		{"cpu_model", cpu.ModelName},
		{"cpu_usage", formatPercent(cpu.UsagePercent)},
		{"memory_total", formatBytes(mem.TotalBytes)},
		{"memory_used", formatPercent(mem.UsedPercent)},
	}
	if body.Load != nil {
		l := body.Load
		rows = append(rows, []string{"load_1", strconv.FormatFloat(l.Load1, 'f', 2, 64)})
		rows = append(rows, []string{"load_5", strconv.FormatFloat(l.Load5, 'f', 2, 64)})
		rows = append(rows, []string{"load_15", strconv.FormatFloat(l.Load15, 'f', 2, 64)})
	}
	return writeKVTable(rows)
}

func writeWorkloadsHuman(items []api.WorkloadItem) error {
	rows := make([][]string, 0, len(items))
	for _, w := range items {
		node := w.Node
		if node == "" {
			node = "-"
		}
		rows = append(rows, []string{w.Name, node, w.Runtime, statusBadge(w.Status), w.Image})
	}
	return writeTable([]string{"NAME", "NODE", "RUNTIME", "STATUS", "IMAGE"}, rows)
}

func writeAssignmentsHuman(items []api.AssignmentItem) error {
	rows := make([][]string, 0, len(items))
	for _, a := range items {
		node := nonEmpty(a.Node)
		image := nonEmpty(a.Image)
		errMsg := nonEmpty(a.Error)
		rows = append(rows, []string{a.Key, node, string(a.Runtime), statusBadge(a.Status), image, errMsg})
	}
	return writeTable([]string{"KEY", "NODE", "RUNTIME", "STATUS", "IMAGE", "ERROR"}, rows)
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
