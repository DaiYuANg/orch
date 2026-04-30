package cmd

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

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
			return cliapp.RunManifest(contextFromCmd(cmd), func(ctx context.Context, lg *slog.Logger, _ *loader.Loader) error {
				_ = ctx
				if file == "" {
					return oopsx.B("cli").Errorf("--file is required")
				}
				res, err := composeimport.LoadComposeFile(context.Background(), file)
				if err != nil {
					return err
				}
				if !quiet {
					for _, w := range res.Report.Warnings {
						lg.Warn("compose import warning", "detail", w)
					}
				}
				if err := res.App.Validate(); err != nil {
					return oopsx.B("cli").Wrapf(err, "validate imported App")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(res.App)
				}
				enc := yaml.NewEncoder(os.Stdout)
				enc.SetIndent(2)
				if err := enc.Encode(res.App); err != nil {
					return oopsx.B("cli").Wrapf(err, "encode YAML")
				}
				return enc.Close()
			})
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to docker-compose.yaml (or compose.yml)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print canonical App as JSON instead of YAML")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress warning log lines for unmapped Compose fields")
	return cmd
}
