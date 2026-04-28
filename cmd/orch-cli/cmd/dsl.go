package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/oopsx"
)

func newDSLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dsl",
		Short: "DSL and deployment file tooling",
	}

	cmd.AddCommand(newDSLParseCmd())
	return cmd
}

func newDSLParseCmd() *cobra.Command {
	var file string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Parse an orch deploy YAML into canonical model",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to deploy YAML file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output parsed model as JSON")
	return cmd
}
