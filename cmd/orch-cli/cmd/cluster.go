package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
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

func newAppsCmd() *cobra.Command {
	return newAppsListCmd("apps", []string{"app"})
}

func newAppsListCmd(use string, aliases []string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   "List deployed apps",
		Long:    `Shows desired apps with status aggregated from workload assignments.`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			return runListApps(ctx, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
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
	cmd.AddCommand(newAppsListCmd("apps", []string{"app"}))
	cmd.AddCommand(newWorkloadsListCmd("workloads", []string{"workload", "wl", "ps"}))
	cmd.AddCommand(newAssignmentsListCmd("assignments", []string{"assignment", "assign"}))
	return cmd
}

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show detailed cluster resource state",
		Long:  `Show detailed resource state from the current control plane context (--server).`,
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newDescribeAppCmd())
	return cmd
}

func newDescribeAppCmd() *cobra.Command {
	var namespace string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "app NAME",
		Short: "Describe an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.GetApp(ctx, namespace, args[0])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "describe app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeAppDetailHuman(&out.Body)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRaftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raft",
		Short: "Inspect and manage Raft membership",
		Long:  `Raft membership commands operate on the configured control plane. Write operations must target the current Raft leader.`,
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newRaftStatusCmd())
	cmd.AddCommand(newRaftMembersCmd())
	cmd.AddCommand(newRaftAddVoterCmd())
	cmd.AddCommand(newRaftRemoveVoterCmd())
	return cmd
}

func newRaftStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local Raft state and leader identity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.RaftStatus(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "raft status")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeRaftStatusHuman(out)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRaftMembersCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "members",
		Aliases: []string{"member", "peers"},
		Short:   "List Raft members",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.ListRaftMembers(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "list raft members")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body.Items)
				}
				return writeRaftMembersHuman(out.Body.Items)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newRaftAddVoterCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "add-voter ID ADDRESS",
		Short: "Add or update a Raft voter",
		Long:  `Adds or updates a Raft voter by node ID and advertised raft host:port. Target the current Raft leader.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.AddRaftVoter(ctx, args[0], args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "add raft voter")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("raft",
					viewField("status", statusBadge("accepted")),
					viewField("id", out.Body.Member.ID),
					viewField("address", out.Body.Member.Address),
				)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRaftRemoveVoterCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "remove-voter ID",
		Aliases: []string{"remove-member"},
		Short:   "Remove a Raft member",
		Long:    `Removes a Raft server from membership. Target the current Raft leader.`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.RemoveRaftMember(ctx, args[0])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "remove raft member")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("raft",
					viewField("status", statusBadge("accepted")),
					viewField("id", out.Body.ID),
				)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newDeleteCmd(use string, aliases []string) *cobra.Command {
	var namespace string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     use + " app NAME",
		Aliases: aliases,
		Short:   "Stop and delete a deployed app",
		Long:    `Stops workloads for the named app, removes the desired app document, and records stopped assignments.`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "app" {
				return oopsx.B("cli").Errorf("expected resource type app, got %q", args[0])
			}
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.DeleteDeploy(ctx, namespace, args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "delete app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("delete",
					viewField("status", statusBadge(out.Body.Status)),
					viewField("app", out.Body.App),
					viewField("namespace", out.Body.Namespace),
				)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newStartCmd() *cobra.Command {
	var namespace string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "start app NAME",
		Short: "Start a stopped app",
		Long:  `Starts workloads for the named app from the desired app document retained by the control plane.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "app" {
				return oopsx.B("cli").Errorf("expected resource type app, got %q", args[0])
			}
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.StartDeploy(ctx, namespace, args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "start app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("start",
					viewField("status", statusBadge(out.Body.Status)),
					viewField("app", out.Body.App),
					viewField("namespace", out.Body.Namespace),
				)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newStopCmd() *cobra.Command {
	var namespace string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "stop app NAME",
		Short: "Stop a deployed app",
		Long:  `Stops workloads for the named app and records stopped assignments while keeping the desired app document.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "app" {
				return oopsx.B("cli").Errorf("expected resource type app, got %q", args[0])
			}
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.StopDeploy(ctx, namespace, args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "stop app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("stop",
					viewField("status", statusBadge(out.Body.Status)),
					viewField("app", out.Body.App),
					viewField("namespace", out.Body.Namespace),
				)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRestartCmd() *cobra.Command {
	var namespace string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "restart app NAME",
		Short: "Restart a deployed app",
		Long:  `Stops then starts workloads for the named app using its desired app document.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "app" {
				return oopsx.B("cli").Errorf("expected resource type app, got %q", args[0])
			}
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.RestartDeploy(ctx, namespace, args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "restart app")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("restart",
					viewField("status", statusBadge(out.Body.Status)),
					viewField("app", out.Body.App),
					viewField("namespace", out.Body.Namespace),
				)
			})
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "App namespace")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
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

func runListApps(ctx context.Context, jsonOut bool) error {
	conn := cliapp.ConnFromGlobals(serverURL, authToken)
	return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
		out, err := c.ListApps(ctx)
		if err != nil {
			return oopsx.B("cli").Wrapf(err, "list apps")
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out.Body.Items)
		}
		return writeAppsHuman(out.Body.Items)
	})
}

type applyWatchOutput struct {
	Accepted    bool                           `json:"accepted"`
	App         string                         `json:"app"`
	Workloads   int                            `json:"workloads"`
	Assignments *list.List[api.AssignmentItem] `json:"assignments"`
	Registry    *list.List[api.WorkloadItem]   `json:"registry"`
}

type deploySnapshot struct {
	Assignments        *list.List[api.AssignmentItem]
	Workloads          *list.List[api.WorkloadItem]
	Total              int
	RunningAssignments int
	RunningWorkloads   int
	FailedAssignment   *api.AssignmentItem
}

func newDeploySnapshot(total int) *deploySnapshot {
	return &deploySnapshot{
		Assignments: list.NewList[api.AssignmentItem](),
		Workloads:   list.NewList[api.WorkloadItem](),
		Total:       total,
	}
}

func watchDeployment(ctx context.Context, c *apiclient.Client, app *deployv1.App, timeout time.Duration, progress bool) (*deploySnapshot, error) {
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	expectedKeys, expectedNames := expectedDeployWorkloads(app)
	if expectedKeys.Len() == 0 {
		return newDeploySnapshot(0), nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	spinner := startWatchSpinner(progress, expectedKeys.Len())
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var last *deploySnapshot
	var lastErr error
	for {
		snapshot, err := readDeploySnapshot(waitCtx, c, expectedKeys, expectedNames)
		if err != nil {
			lastErr = err
			updateWatchSpinner(spinner, last, expectedKeys.Len(), err)
		} else {
			last = snapshot
			updateWatchSpinner(spinner, snapshot, expectedKeys.Len(), nil)
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
			failWatchSpinner(spinner, watchStatusText(last, expectedKeys.Len(), "timed out"))
			if lastErr != nil {
				return last, oopsx.B("cli").Wrapf(lastErr, "wait for deploy status timed out after %s", timeout)
			}
			return last, oopsx.B("cli").Errorf("wait for deploy status timed out after %s: %s", timeout, watchStatusText(last, expectedKeys.Len(), ""))
		case <-ticker.C:
		}
	}
}

func expectedDeployWorkloads(app *deployv1.App) (*set.Set[string], *set.Set[string]) {
	keys := set.NewSet[string]()
	names := set.NewSet[string]()
	if app == nil {
		return keys, names
	}
	for _, workload := range app.Workloads {
		key := workloadmeta.AssignmentKey(app.Metadata, workload.Name)
		if key == "" {
			continue
		}
		keys.Add(key)
		names.Add(workload.Name)
	}
	return keys, names
}

func readDeploySnapshot(ctx context.Context, c *apiclient.Client, expectedKeys, expectedNames *set.Set[string]) (*deploySnapshot, error) {
	assignments, err := c.ListAssignments(ctx)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "list assignments")
	}
	workloads, err := c.ListWorkloads(ctx)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "list workloads")
	}

	snapshot := newDeploySnapshot(expectedKeys.Len())
	assignments.Body.Items.Range(func(_ int, assignment api.AssignmentItem) bool {
		if !expectedKeys.Contains(assignment.Key) {
			return true
		}
		snapshot.Assignments.Add(assignment)
		if assignment.Status == workloadmeta.AssignmentStatusRunning {
			snapshot.RunningAssignments++
		}
		if assignment.Status == workloadmeta.AssignmentStatusFailed && snapshot.FailedAssignment == nil {
			failed := assignment
			snapshot.FailedAssignment = &failed
		}
		return true
	})
	workloads.Body.Items.Range(func(_ int, workload api.WorkloadItem) bool {
		if !expectedNames.Contains(workload.Name) {
			return true
		}
		snapshot.Workloads.Add(workload)
		if workload.Status == "running" {
			snapshot.RunningWorkloads++
		}
		return true
	})
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
	rows := list.NewGrid[string](
		[]string{"hostname", h.Hostname},
		[]string{"os", h.OS},
		[]string{"platform", h.Platform},
		[]string{"kernel", h.KernelVersion},
		[]string{"arch", h.KernelArch},
		[]string{"cpu_cores", strconv.Itoa(cpu.LogicalCores)},
		[]string{"cpu_model", cpu.ModelName},
		[]string{"cpu_usage", formatPercent(cpu.UsagePercent)},
		[]string{"memory_total", formatBytes(mem.TotalBytes)},
		[]string{"memory_used", formatPercent(mem.UsedPercent)},
	)
	if body.Load != nil {
		l := body.Load
		rows.AddRow("load_1", strconv.FormatFloat(l.Load1, 'f', 2, 64))
		rows.AddRow("load_5", strconv.FormatFloat(l.Load5, 'f', 2, 64))
		rows.AddRow("load_15", strconv.FormatFloat(l.Load15, 'f', 2, 64))
	}
	return writeKVTable(rows)
}

func writeWorkloadsHuman(items *list.List[api.WorkloadItem]) error {
	rows := list.NewGridWithCapacity[string](items.Len())
	items.Range(func(_ int, w api.WorkloadItem) bool {
		node := w.Node
		if node == "" {
			node = "-"
		}
		rows.AddRow(w.Name, node, w.Runtime, statusBadge(w.Status), w.Artifact)
		return true
	})
	return writeTable(list.NewList("NAME", "NODE", "RUNTIME", "STATUS", "ARTIFACT"), rows)
}

func writeAssignmentsHuman(items *list.List[api.AssignmentItem]) error {
	rows := list.NewGridWithCapacity[string](items.Len())
	items.Range(func(_ int, a api.AssignmentItem) bool {
		node := nonEmpty(a.Node)
		artifact := nonEmpty(a.Artifact)
		errMsg := nonEmpty(a.Error)
		rows.AddRow(a.Key, node, string(a.Runtime), statusBadge(a.Status), artifact, errMsg)
		return true
	})
	return writeTable(list.NewList("KEY", "NODE", "RUNTIME", "STATUS", "ARTIFACT", "ERROR"), rows)
}

func writeAppsHuman(items *list.List[api.AppItem]) error {
	rows := list.NewGridWithCapacity[string](items.Len())
	items.Range(func(_ int, app api.AppItem) bool {
		rows.AddRow(app.Namespace, app.Name, statusBadge(app.Status), appReadyText(app.Running, app.DesiredWorkloads), nonEmpty(app.DesiredGeneration), nonEmpty(app.ObservedGeneration), appCountsText(app), formatTime(app.LastTransitionAt), nonEmpty(app.LastError))
		return true
	})
	return writeTable(list.NewList("NAMESPACE", "NAME", "STATUS", "READY", "GENERATION", "OBSERVED", "COUNTS", "UPDATED", "ERROR"), rows)
}

func writeAppDetailHuman(app *api.AppDetailItem) error {
	if app == nil {
		return writeLine(viewMutedStyle.Render("No resources found."))
	}
	rows := list.NewGrid[string](
		[]string{"namespace", app.Namespace},
		[]string{"name", app.Name},
		[]string{"status", statusBadge(app.Status)},
		[]string{"generation", nonEmpty(app.DesiredGeneration)},
		[]string{"observed_generation", nonEmpty(app.ObservedGeneration)},
		[]string{"ready", appReadyText(app.Running, app.DesiredWorkloads)},
		[]string{"workloads", strconv.Itoa(app.DesiredWorkloads)},
		[]string{"running", strconv.Itoa(app.Running)},
		[]string{"stopped", strconv.Itoa(app.Stopped)},
		[]string{"failed", strconv.Itoa(app.Failed)},
		[]string{"pending", strconv.Itoa(app.Pending)},
		[]string{"last_transition", formatTime(app.LastTransitionAt)},
		[]string{"last_error", nonEmpty(app.LastError)},
	)
	if err := writeKVTable(rows); err != nil {
		return err
	}
	if err := writeLine(""); err != nil {
		return err
	}
	workloadRows := list.NewGridWithCapacity[string](app.Workloads.Len())
	app.Workloads.Range(func(_ int, workload api.AppWorkloadItem) bool {
		workloadRows.AddRow(workload.Name, string(workload.Kind), string(workload.Runtime), nonEmpty(workload.Node), statusBadge(workload.Status), nonEmpty(workload.Generation), nonEmpty(workload.Artifact), nonEmpty(workload.Error))
		return true
	})
	return writeTable(list.NewList("WORKLOAD", "KIND", "RUNTIME", "NODE", "STATUS", "GENERATION", "ARTIFACT", "ERROR"), workloadRows)
}

func appReadyText(running, total int) string {
	return strconv.Itoa(running) + "/" + strconv.Itoa(total)
}

func appCountsText(app api.AppItem) string {
	return fmt.Sprintf("run=%d stop=%d fail=%d pending=%d", app.Running, app.Stopped, app.Failed, app.Pending)
}

func writeRaftStatusHuman(out *api.RaftStatusOutput) error {
	body := out.Body
	role := "follower"
	if body.IsLeader {
		role = "leader"
	}
	if !body.Ready {
		role = "-"
	}
	memberCount := 0
	if body.Members != nil {
		memberCount = body.Members.Len()
	}
	rows := list.NewGrid[string](
		[]string{"enabled", strconv.FormatBool(body.Enabled)},
		[]string{"ready", strconv.FormatBool(body.Ready)},
		[]string{"state", statusBadge(body.State)},
		[]string{"role", role},
		[]string{"node_id", nonEmpty(body.NodeID)},
		[]string{"leader_id", nonEmpty(body.LeaderID)},
		[]string{"leader_address", nonEmpty(body.LeaderAddress)},
		[]string{"local_address", nonEmpty(body.LocalAddress)},
		[]string{"members", strconv.Itoa(memberCount)},
	)
	return writeKVTable(rows)
}

func writeRaftMembersHuman(items *list.List[api.RaftMemberItem]) error {
	rows := list.NewGridWithCapacity[string](items.Len())
	items.Range(func(_ int, member api.RaftMemberItem) bool {
		rows.AddRow(member.ID, member.Address, member.Suffrage)
		return true
	})
	return writeTable(list.NewList("ID", "ADDRESS", "SUFFRAGE"), rows)
}

func nonEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func contextFromCmd(cmd *cobra.Command) context.Context {
	if cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}
