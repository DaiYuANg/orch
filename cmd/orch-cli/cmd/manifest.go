package cmd

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func loadValidatedManifest(file string) (*deployv1.App, error) {
	if file == "" {
		return nil, oopsx.B("cli").Errorf("--file is required")
	}
	app, err := deployv1.LoadAppFile(file)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "load manifest")
	}
	if err := app.Validate(); err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "validate manifest")
	}
	return app, nil
}

func runValidateManifest(file string, lg *slog.Logger) error {
	app, err := loadValidatedManifest(file)
	if err != nil {
		return err
	}
	if err := fprintfStdout("OK app=%s namespace=%s\n", app.Metadata.Name, app.Metadata.Namespace); err != nil {
		return err
	}
	lg.Debug("manifest validated", "app", app.Metadata.Name, "namespace", app.Metadata.Namespace)
	return nil
}

func runParseManifest(file string, jsonOut bool, lg *slog.Logger) error {
	app, err := loadValidatedManifest(file)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(app); err != nil {
			return oopsx.B("cli").Wrapf(err, "encode manifest JSON")
		}
		return nil
	}
	if err := fprintfStdout("app=%s namespace=%s workloads=%d ingresses=%d volumes=%d configs=%d secrets=%d\n",
		app.Metadata.Name,
		app.Metadata.Namespace,
		len(app.Workloads),
		len(app.Ingresses),
		len(app.Volumes),
		len(app.Configs),
		len(app.Secrets),
	); err != nil {
		return err
	}
	lg.Debug("manifest parsed", "app", app.Metadata.Name)
	return nil
}

func newValidateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a deploy YAML without contacting the server",
		Long:  `Runs the same parse and validation rules the control plane uses. Exit 0 if the manifest is OK (useful in CI).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, lg *slog.Logger) error {
				_ = ctx
				return runValidateManifest(file, lg)
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
				return runParseManifest(file, jsonOut, lg)
			})
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output parsed model as JSON")
	return cmd
}
