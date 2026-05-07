package cmd

import (
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/buildmeta"
)

var (
	serverURL string
	authToken string
)

var rootCmd = &cobra.Command{
	Use:           "orch",
	Short:         "orch CLI — deploy manifests and operate the orch control plane",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       buildmeta.Version(),
	Long: `Validate and inspect deploy YAML locally (validate, parse), submit manifests to the cluster (apply),
and talk to the running control plane (--server / ORCH_SERVER): health, workloads, assignments, hostinfo.

Use a single base URL per process; in clusters you can point at a load balancer or any peer node.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", apiclient.DefaultBaseURL(),
		`Base URL of the orch control plane (no trailing slash). Example: http://127.0.0.1:17443. Env ORCH_SERVER.`)
	rootCmd.PersistentFlags().StringVar(&authToken, "token", os.Getenv("ORCH_TOKEN"),
		`Bearer token when orch-server auth is enabled (env ORCH_TOKEN).`)

	// Manifest workflow (local vs server) — primary user surface.
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newParseCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(newComposeCmd())
	// Cluster inspection (requires reachable control plane).
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newHealthCmd())
	rootCmd.AddCommand(newWorkloadsCmd())
	rootCmd.AddCommand(newAssignmentsCmd())
	rootCmd.AddCommand(newHostinfoCmd())
}
