package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/apiclient"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/oopsx"
)

func newAPICmds() *cobra.Command {
	api := &cobra.Command{
		Use:   "api",
		Short: "Talk to orch-server HTTP API (single --server URL; use LB or any node for clusters)",
	}

	api.AddCommand(newHealthCmd())
	api.AddCommand(newWorkloadsCmd())
	api.AddCommand(newDeployApplyCmd())
	return api
}

func newHealthCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "GET /api/health — verify connectivity to orch-server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			c, err := apiclient.New(serverURL, authToken)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			out, err := c.Health(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(os.Stdout, "status=%s time=%s\n", out.Body.Status, out.Body.Timestamp)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newWorkloadsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "workloads",
		Short: "GET /api/v1/workloads — list registered workloads on this server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			c, err := apiclient.New(serverURL, authToken)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			out, err := c.ListWorkloads(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out.Body.Items)
			}
			fmt.Fprintf(os.Stdout, "NAME\tRUNTIME\tSTATUS\tIMAGE\n")
			for _, w := range out.Body.Items {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", w.Name, w.Runtime, w.Status, w.Image)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newDeployApplyCmd() *cobra.Command {
	var file string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "POST /api/v1/deploy — submit deploy YAML to server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return oopsx.B("cli").Errorf("--file is required")
			}
			app, err := deployv1.LoadAppFile(file)
			if err != nil {
				return err
			}
			ctx := contextFromCmd(cmd)
			c, err := apiclient.New(serverURL, authToken)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			out, err := c.Deploy(ctx, app)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(os.Stdout, "accepted app=%s workloads=%d\n", out.Body.App, out.Body.Workloads)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON response")
	return cmd
}

// topLevelSimpleCmds wires one-shot commands that need context (root may not pass it in older cobra — use Background if nil).
func contextFromCmd(cmd *cobra.Command) context.Context {
	if cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}
