package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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
			return runLogsCommand(cmd, args[0], namespace, app, tail, jsonOut)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().StringVar(&app, "app", "", "App name")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of log lines")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	mustMarkFlagRequired(cmd, "app")
	return cmd
}

func runLogsCommand(cmd *cobra.Command, workloadName, namespace, app string, tail int, jsonOut bool) error {
	ctx := contextFromCmd(cmd)
	conn := cliapp.ConnFromGlobals(serverURL, authToken)
	if err := cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
		out, err := c.WorkloadLogs(ctx, namespace, app, workloadName, tail)
		if err != nil {
			return oopsx.B("cli").Wrapf(err, "logs")
		}
		return writeLogsOutput(out, jsonOut)
	}); err != nil {
		return oopsx.B("cli").Wrapf(err, "run logs")
	}
	return nil
}

func writeLogsOutput(out *api.WorkloadLogsOutput, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out.Body); err != nil {
			return oopsx.B("cli").Wrapf(err, "write logs json")
		}
		return nil
	}
	if strings.TrimSpace(out.Body.Content) == "" {
		return writeLine(viewMutedStyle.Render("No logs."))
	}
	if _, err := fmt.Fprint(os.Stdout, out.Body.Content); err != nil {
		return oopsx.B("cli").Wrapf(err, "write logs")
	}
	return nil
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
			return runDescribeNodeCommand(cmd, args, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func runDescribeNodeCommand(cmd *cobra.Command, args []string, jsonOut bool) error {
	ctx := contextFromCmd(cmd)
	conn := cliapp.ConnFromGlobals(serverURL, authToken)
	if err := cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
		raft, err := c.RaftStatus(ctx)
		if err != nil {
			return oopsx.B("cli").Wrapf(err, "node raft status")
		}
		return writeDescribeNodeOutput(ctx, c, args, raft, jsonOut)
	}); err != nil {
		return oopsx.B("cli").Wrapf(err, "describe node")
	}
	return nil
}

func writeDescribeNodeOutput(
	ctx context.Context,
	c *apiclient.Client,
	args []string,
	raft *api.RaftStatusOutput,
	jsonOut bool,
) error {
	name := describeNodeName(args, raft.Body.NodeID)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(raft.Body); err != nil {
			return oopsx.B("cli").Wrapf(err, "write node json")
		}
		return nil
	}
	if name != "" && name != raft.Body.NodeID {
		return writeRaftMemberNodeHuman(name, raft)
	}
	host, err := c.Hostinfo(ctx)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "node hostinfo")
	}
	return writeLocalNodeHuman(name, raft, host)
}

func describeNodeName(args []string, fallback string) string {
	if len(args) > 0 {
		return strings.TrimSpace(args[0])
	}
	return strings.TrimSpace(fallback)
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
	mustMarkFlagRequired(cmd, "app")
	return cmd
}
