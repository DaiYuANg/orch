package cmd

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/arcgolabs/collectionx/list"
	"github.com/spf13/cobra"

	"github.com/lyonbrown4d/orch/cmd/orch-cli/cliapp"
	"github.com/lyonbrown4d/orch/internal/api"
	"github.com/lyonbrown4d/orch/internal/apiclient"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/dixdiag"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func newDiagnosticsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "diagnostics",
		Aliases: []string{"diag"},
		Short:   "Show control-plane runtime diagnostics",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.Diagnostics(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "diagnostics")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeDiagnosticsHuman(out)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func writeDiagnosticsHuman(out *api.DiagnosticsOutput) error {
	if out == nil {
		return writeLine(viewMutedStyle.Render("No diagnostics response."))
	}
	body := out.Body
	if err := writeInfoLine("diagnostics",
		viewField("app", nonEmpty(body.App.Name)),
		viewField("state", statusBadge(body.App.State)),
		viewField("version", nonEmpty(body.App.Version)),
		viewField("build", nonEmpty(body.App.BuildDuration)),
		viewField("start", nonEmpty(body.App.StartDuration)),
		viewField("events", strconv.Itoa(body.Events.Count)+"/"+strconv.Itoa(body.Events.Capacity)),
		viewField("graph", strconv.Itoa(body.Graph.Nodes)+"/"+strconv.Itoa(body.Graph.Edges)),
	); err != nil {
		return err
	}
	if err := writeLine(""); err != nil {
		return err
	}
	if err := writeDiagnosticsLifecycle(body.Lifecycle); err != nil {
		return err
	}
	if err := writeLine(""); err != nil {
		return err
	}
	return writeDiagnosticsEvents(body.Events.Recent)
}

func writeDiagnosticsLifecycle(lc dixdiag.LifecycleSnapshot) error {
	rows := list.NewGridWithCapacity[string](lc.StartHooks + lc.StopHooks)
	addHookRows := func(phase string, hooks *list.List[dixdiag.HookInfo]) {
		if hooks == nil {
			return
		}
		hooks.Range(func(_ int, hook dixdiag.HookInfo) bool {
			name := hook.Name
			if name == "" {
				name = hook.Label
			}
			rows.AddRow(
				phase,
				nonEmpty(name),
				strconv.Itoa(hook.Priority),
				strconv.FormatBool(hook.Parallel),
				nonEmpty(hook.Timeout),
			)
			return true
		})
	}
	addHookRows("start", lc.Start)
	addHookRows("stop", lc.Stop)
	return writeTable(list.NewList("PHASE", "HOOK", "PRIORITY", "PARALLEL", "TIMEOUT"), rows)
}

func writeDiagnosticsEvents(events *list.List[dixdiag.RecentEvent]) error {
	if events == nil || events.Len() == 0 {
		return writeLine(viewMutedStyle.Render("No recent dix events."))
	}
	values := events.Values()
	start := max(len(values)-20, 0)
	rows := list.NewGridWithCapacity[string](len(values) - start)
	for i := start; i < len(values); i++ {
		event := &values[i]
		rows.AddRow(
			formatTime(event.At),
			event.Type,
			nonEmpty(event.Operation),
			nonEmpty(event.Target),
			statusBadge(event.Status),
			nonEmpty(event.Duration),
			nonEmpty(event.Detail),
		)
	}
	return writeTable(list.NewList("TIME", "TYPE", "OP", "TARGET", "STATUS", "DURATION", "DETAIL"), rows)
}
