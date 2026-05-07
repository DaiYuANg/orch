package cmd

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/deploy/loader"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func loadValidatedManifest(ctx context.Context, deploy *loader.Loader, file string) (*deployv1.App, error) {
	if file == "" {
		return nil, oopsx.B("cli").Errorf("--file is required")
	}
	if deploy == nil {
		return nil, oopsx.B("cli").Errorf("deploy loader is nil")
	}
	app, err := deploy.LoadApp(ctx, file)
	if err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "load manifest")
	}
	if err := app.Validate(); err != nil {
		return nil, oopsx.B("cli").Wrapf(err, "validate manifest")
	}
	return app, nil
}

func runValidateManifest(ctx context.Context, deploy *loader.Loader, file string) error {
	app, err := loadValidatedManifest(ctx, deploy, file)
	if err != nil {
		return err
	}
	return writeInfoLine("validate",
		viewField("status", statusBadge("ok")),
		viewField("app", app.Metadata.Name),
		viewField("namespace", app.Metadata.Namespace),
	)
}

func runParseManifest(ctx context.Context, deploy *loader.Loader, file string, jsonOut bool) error {
	app, err := loadValidatedManifest(ctx, deploy, file)
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
	return writeInfoLine("manifest",
		viewField("app", app.Metadata.Name),
		viewField("namespace", app.Metadata.Namespace),
		viewField("workloads", strconv.Itoa(len(app.Workloads))),
		viewField("ingresses", strconv.Itoa(len(app.Ingresses))),
		viewField("volumes", strconv.Itoa(len(app.Volumes))),
		viewField("configs", strconv.Itoa(len(app.Configs))),
		viewField("secrets", strconv.Itoa(len(app.Secrets))),
	)
}

func newValidateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a deploy manifest (.orch or YAML) without contacting the server",
		Long:  `Runs the same parse and validation rules the control plane uses. Exit 0 if the manifest is OK (useful in CI).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, deploy *loader.Loader) error {
				return runValidateManifest(ctx, deploy, file)
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy file (.orch or YAML)")
	return cmd
}

func newParseCmd() *cobra.Command {
	var file string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse a deploy YAML and print a summary or the canonical JSON model",
		Long:  `Loads the file (.orch or YAML), validates it, then prints either a one-line summary or the full structured app document with --json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, deploy *loader.Loader) error {
				return runParseManifest(ctx, deploy, file, jsonOut)
			})
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy file (.orch or YAML)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output parsed model as JSON")
	return cmd
}
