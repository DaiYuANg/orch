package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newValidateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a deploy YAML without contacting the server",
		Long:  `Runs the same parse and validation rules the control plane uses. Exit 0 if the manifest is OK (useful in CI).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, lg *slog.Logger) error {
				_ = ctx
				if file == "" {
					return oopsx.B("cli").Errorf("--file is required")
				}
				app, err := deployv1.LoadAppFile(file)
				if err != nil {
					return err
				}
				if err := app.Validate(); err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "OK app=%s namespace=%s\n", app.Metadata.Name, app.Metadata.Namespace)
				lg.Debug("manifest validated", "app", app.Metadata.Name, "namespace", app.Metadata.Namespace)
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML file")
	return cmd
}

func newParseCmd() *cobra.Command {
	var file string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse a deploy YAML and print a summary or the canonical JSON model",
		Long:  `Loads the file, validates it, then prints either a one-line summary or the full structured app document with --json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, lg *slog.Logger) error {
				_ = ctx
				if file == "" {
					return oopsx.B("cli").Errorf("--file is required")
				}

				app, err := deployv1.LoadAppFile(file)
				if err != nil {
					return err
				}
				if err := app.Validate(); err != nil {
					return err
				}

				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(app)
				}

				fmt.Fprintf(os.Stdout, "app=%s namespace=%s workloads=%d ingresses=%d volumes=%d configs=%d secrets=%d\n",
					app.Metadata.Name,
					app.Metadata.Namespace,
					len(app.Workloads),
					len(app.Ingresses),
					len(app.Volumes),
					len(app.Configs),
					len(app.Secrets),
				)
				lg.Debug("manifest parsed", "app", app.Metadata.Name)
				return nil
			})
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output parsed model as JSON")
	return cmd
}
