package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/apiclient"
)

var (
	serverURL string
	authToken string
)

var rootCmd = &cobra.Command{
	Use:   "orch",
	Short: "orch CLI — local tooling and orch-server HTTP API",
	Long: `orch-cli talks to orch-server using a single HTTP base URL (--server / ORCH_SERVER).

v1 keeps one endpoint so you can aim at a load balancer or any cluster peer; richer discovery/TLS profiles come later.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", apiclient.DefaultBaseURL(),
		`Base URL of orch-server HTTP API (no trailing slash). Example: http://127.0.0.1:17443 or https://orch.example.com. Override with env ORCH_SERVER.`)
	rootCmd.PersistentFlags().StringVar(&authToken, "token", os.Getenv("ORCH_TOKEN"),
		`Bearer token when orch-server auth is enabled (env ORCH_TOKEN).`)

	rootCmd.AddCommand(newDSLCmd())
	rootCmd.AddCommand(newAPICmds())
}

