package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newReadyCmd() *cobra.Command {
	var jsonOut bool
	var wait bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Check whether the control plane is ready for cluster operations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				var out *api.ReadyOutput
				var err error
				if wait {
					out, err = waitReady(ctx, c, timeout, !jsonOut)
				} else {
					out, err = c.Ready(ctx)
				}
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "ready")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeReadyHuman(out)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait until ready")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Maximum time to wait with --wait")
	return cmd
}

func newWaitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for cluster resources to reach a status",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newWaitAppCmd())
	return cmd
}

func newWaitAppCmd() *cobra.Command {
	var namespace string
	var targetStatus string
	var timeout time.Duration
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "app NAME",
		Short: "Wait for an app status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := waitAppStatus(ctx, c, namespace, args[0], targetStatus, timeout, !jsonOut)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "wait app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("wait",
					viewField("app", out.Body.Name),
					viewField("namespace", out.Body.Namespace),
					viewField("status", statusBadge(out.Body.Status)),
					viewField("ready", appReadyText(out.Body.Running, out.Body.DesiredWorkloads)),
				)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().StringVar(&targetStatus, "for", "running", "Target app status")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Maximum time to wait")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newLogsCmd() *cobra.Command {
	var namespace string
	var app string
	var tail int
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "logs WORKLOAD --app APP",
		Aliases: []string{"log"},
		Short:   "Print workload logs from the assigned runtime node",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.WorkloadLogs(ctx, namespace, app, args[0], tail)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "logs")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				if strings.TrimSpace(out.Body.Content) == "" {
					return writeLine(viewMutedStyle.Render("No logs."))
				}
				_, err = fmt.Fprint(os.Stdout, out.Body.Content)
				return err
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().StringVar(&app, "app", "", "App name")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of log lines")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}

func newEventsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "events",
		Aliases: []string{"event"},
		Short:   "Show recent assignment events",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.ListAssignments(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "events")
				}
				items := out.Body.Items.Values()
				sort.SliceStable(items, func(i, j int) bool {
					return items[i].UpdatedAt.After(items[j].UpdatedAt)
				})
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(items)
				}
				return writeEventsHuman(items)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newDescribeNodeCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "node [NAME]",
		Short: "Describe a control-plane node",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				raft, err := c.RaftStatus(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "node raft status")
				}
				name := strings.TrimSpace(raft.Body.NodeID)
				if len(args) > 0 {
					name = strings.TrimSpace(args[0])
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(raft.Body)
				}
				if name == "" || name == raft.Body.NodeID {
					host, err := c.Hostinfo(ctx)
					if err != nil {
						return oopsx.B("cli").Wrapf(err, "node hostinfo")
					}
					return writeLocalNodeHuman(name, raft, host)
				}
				return writeRaftMemberNodeHuman(name, raft)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newDescribeWorkloadCmd() *cobra.Command {
	var namespace string
	var app string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "workload NAME --app APP",
		Aliases: []string{"wl"},
		Short:   "Describe runtime-local workload status",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.WorkloadRuntimeStatus(ctx, namespace, app, args[0])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "describe workload")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeWorkloadRuntimeStatusHuman(out)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().StringVar(&app, "app", "", "App name")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}

func waitReady(ctx context.Context, c *apiclient.Client, timeout time.Duration, progress bool) (*api.ReadyOutput, error) {
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	spinner := startStatusSpinner(progress, "waiting for control plane readiness")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var last *api.ReadyOutput
	var lastErr error
	for {
		out, err := c.Ready(waitCtx)
		if err != nil {
			lastErr = err
			updateStatusSpinner(spinner, "waiting for control plane readiness last_error="+err.Error())
		} else {
			last = out
			updateStatusSpinner(spinner, "waiting for control plane readiness status="+out.Body.Status)
			if out.Body.Ready {
				successWatchSpinner(spinner, "control plane ready")
				return out, nil
			}
		}

		select {
		case <-waitCtx.Done():
			failWatchSpinner(spinner, "control plane readiness timed out")
			if lastErr != nil {
				return last, oopsx.B("cli").Wrapf(lastErr, "wait ready timed out after %s", timeout)
			}
			return last, oopsx.B("cli").Errorf("wait ready timed out after %s", timeout)
		case <-ticker.C:
		}
	}
}

func waitAppStatus(ctx context.Context, c *apiclient.Client, namespace, name, target string, timeout time.Duration, progress bool) (*api.GetAppOutput, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "running"
	}
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	spinner := startStatusSpinner(progress, "waiting for app "+name+" status="+target)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var last *api.GetAppOutput
	var lastErr error
	for {
		out, err := c.GetApp(waitCtx, namespace, name)
		if err != nil {
			lastErr = err
			updateStatusSpinner(spinner, "waiting for app "+name+" last_error="+err.Error())
		} else {
			last = out
			status := strings.ToLower(strings.TrimSpace(out.Body.Status))
			updateStatusSpinner(spinner, "waiting for app "+name+" status="+status+" ready="+appReadyText(out.Body.Running, out.Body.DesiredWorkloads))
			if status == target {
				successWatchSpinner(spinner, "app "+name+" status="+status)
				return out, nil
			}
			if status == "failed" && target != "failed" {
				failWatchSpinner(spinner, "app "+name+" failed")
				return out, oopsx.B("cli").Errorf("app %s reached failed status: %s", name, nonEmpty(out.Body.LastError))
			}
		}

		select {
		case <-waitCtx.Done():
			failWatchSpinner(spinner, "wait app timed out")
			if lastErr != nil {
				return last, oopsx.B("cli").Wrapf(lastErr, "wait app timed out after %s", timeout)
			}
			return last, oopsx.B("cli").Errorf("wait app timed out after %s", timeout)
		case <-ticker.C:
		}
	}
}

func startStatusSpinner(progress bool, text string) *pterm.SpinnerPrinter {
	if !progress || !stderrIsTerminal() {
		return nil
	}
	spinner, err := pterm.DefaultSpinner.WithRemoveWhenDone(false).Start(text)
	if err != nil {
		return nil
	}
	return spinner
}

func updateStatusSpinner(spinner *pterm.SpinnerPrinter, text string) {
	if spinner != nil {
		spinner.UpdateText(text)
	}
}

func writeReadyHuman(out *api.ReadyOutput) error {
	if out == nil {
		return writeLine(viewMutedStyle.Render("No readiness response."))
	}
	if err := writeInfoLine("ready",
		viewField("status", statusBadge(out.Body.Status)),
		viewField("time", out.Body.Timestamp),
	); err != nil {
		return err
	}
	rows := list.NewGridWithCapacity[string](out.Body.Checks.Len())
	out.Body.Checks.Range(func(_ int, check api.ReadyCheckItem) bool {
		rows.AddRow(check.Name, strconv.FormatBool(check.Ready), statusBadge(check.Status), nonEmpty(check.Detail))
		return true
	})
	return writeTable(list.NewList("CHECK", "READY", "STATUS", "DETAIL"), rows)
}

func writeEventsHuman(items []api.AssignmentItem) error {
	rows := list.NewGridWithCapacity[string](len(items))
	for _, item := range items {
		rows.AddRow(formatTime(item.UpdatedAt), item.Key, nonEmpty(item.Node), string(item.Runtime), statusBadge(item.Status), nonEmpty(item.Error))
	}
	return writeTable(list.NewList("TIME", "KEY", "NODE", "RUNTIME", "STATUS", "ERROR"), rows)
}

func writeLocalNodeHuman(name string, raft *api.RaftStatusOutput, host *api.HostinfoOutput) error {
	if raft == nil || host == nil {
		return writeLine(viewMutedStyle.Render("No node response."))
	}
	body := host.Body
	rows := list.NewGrid[string](
		[]string{"node_id", nonEmpty(name)},
		[]string{"hostname", nonEmpty(body.Host.Hostname)},
		[]string{"os", body.Host.OS},
		[]string{"arch", body.Host.KernelArch},
		[]string{"raft_ready", strconv.FormatBool(raft.Body.Ready)},
		[]string{"raft_state", statusBadge(raft.Body.State)},
		[]string{"raft_leader", nonEmpty(raft.Body.LeaderID)},
		[]string{"raft_address", nonEmpty(raft.Body.LocalAddress)},
		[]string{"cpu_cores", strconv.Itoa(body.CPU.LogicalCores)},
		[]string{"cpu_usage", formatPercent(body.CPU.UsagePercent)},
		[]string{"memory_total", formatBytes(body.Memory.TotalBytes)},
		[]string{"memory_used", formatPercent(body.Memory.UsedPercent)},
	)
	return writeKVTable(rows)
}

func writeRaftMemberNodeHuman(name string, raft *api.RaftStatusOutput) error {
	if raft == nil || raft.Body.Members == nil {
		return writeLine(viewMutedStyle.Render("No node response."))
	}
	var found api.RaftMemberItem
	ok := false
	raft.Body.Members.Range(func(_ int, member api.RaftMemberItem) bool {
		if member.ID != name {
			return true
		}
		found = member
		ok = true
		return false
	})
	if !ok {
		return oopsx.B("cli").Errorf("node %q not found in raft members", name)
	}
	rows := list.NewGrid[string](
		[]string{"node_id", found.ID},
		[]string{"raft_address", found.Address},
		[]string{"suffrage", found.Suffrage},
		[]string{"leader", strconv.FormatBool(found.ID == raft.Body.LeaderID)},
		[]string{"local", strconv.FormatBool(found.ID == raft.Body.NodeID)},
	)
	return writeKVTable(rows)
}

func writeWorkloadRuntimeStatusHuman(out *api.WorkloadRuntimeStatusOutput) error {
	if out == nil {
		return writeLine(viewMutedStyle.Render("No workload status response."))
	}
	body := out.Body
	rows := list.NewGrid[string](
		[]string{"name", nonEmpty(body.Name)},
		[]string{"runtime", string(body.Runtime)},
		[]string{"status", statusBadge(body.Status)},
		[]string{"native_id", nonEmpty(body.NativeID)},
		[]string{"started_at", formatTime(body.StartedAt)},
		[]string{"updated_at", formatTime(body.UpdatedAt)},
		[]string{"message", nonEmpty(body.Message)},
	)
	return writeKVTable(rows)
}
