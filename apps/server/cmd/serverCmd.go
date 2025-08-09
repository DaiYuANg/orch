package cmd

import (
	"github.com/DaiYuANg/warden/container"
	"github.com/DaiYuANg/warden/server/internal/auth"
	"github.com/DaiYuANg/warden/server/internal/common"
	"github.com/DaiYuANg/warden/server/internal/config"
	"github.com/DaiYuANg/warden/server/internal/dns"
	"github.com/DaiYuANg/warden/server/internal/endpoint"
	"github.com/DaiYuANg/warden/server/internal/http"
	"github.com/DaiYuANg/warden/server/internal/mdns"
	"github.com/DaiYuANg/warden/server/internal/raft"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var app *fx.App

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	PreRun: func(cmd *cobra.Command, args []string) {
		app = container.CreateContainer(
			config.Module,
			auth.Module,
			mdns.Module,
			raft.Module,
			common.Module,
			endpoint.Module,
			http.Module,
			dns.Module,
		)
	},
	Run: func(cmd *cobra.Command, args []string) {
		app.Run()
	},
}
