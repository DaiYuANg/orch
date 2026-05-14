package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/deploy/composeimport"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newComposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Docker Compose compatibility (imports map into the canonical manifest)",
	}
	cmd.AddCommand(newComposeImportCmd())
	return cmd
}

func newComposeImportCmd() *cobra.Command {
	var file string
	var jsonOut bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Parse docker-compose YAML and emit canonical orch App (stdout)",
		Long: `Loads a Compose file via compose-spec/go, converts services into workloads on runtime=docker,
then prints YAML or JSON. Scheduling and runtime execution always consume deploy/v1alpha1.App — this command is only an importer.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, _ *loader.Loader) error {
				return runComposeImportCommand(ctx, file, jsonOut, quiet)
			})
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to docker-compose.yaml (or compose.yml)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print canonical App as JSON instead of YAML")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress warning lines for unmapped Compose fields")
	return cmd
}

func runComposeImportCommand(ctx context.Context, file string, jsonOut, quiet bool) error {
	if file == "" {
		return oopsx.B("cli").Errorf("--file is required")
	}
	res, err := composeimport.LoadComposeFile(ctx, file)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "import compose file")
	}
	if !quiet {
		for _, w := range res.Report.Warnings {
			pterm.Warning.Println(w)
		}
	}
	if err := res.App.Validate(); err != nil {
		return oopsx.B("cli").Wrapf(err, "validate imported App")
	}
	return writeComposeImportApp(res.App, jsonOut)
}

func writeComposeImportApp(app any, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(app); err != nil {
			return oopsx.B("cli").Wrapf(err, "encode JSON")
		}
		return nil
	}
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	if err := enc.Encode(app); err != nil {
		return oopsx.B("cli").Wrapf(err, "encode YAML")
	}
	if err := enc.Close(); err != nil {
		return oopsx.B("cli").Wrapf(err, "close YAML encoder")
	}
	return nil
}
