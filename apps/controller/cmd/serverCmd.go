package cmd

import (
	"github.com/DaiYuANg/warden/container"
	"github.com/DaiYuANg/warden/controller/internal/auth"
	"github.com/DaiYuANg/warden/controller/internal/common"
	"github.com/DaiYuANg/warden/controller/internal/config"
	"github.com/DaiYuANg/warden/controller/internal/dns"
	"github.com/DaiYuANg/warden/controller/internal/endpoint"
	"github.com/DaiYuANg/warden/controller/internal/http"
	"github.com/DaiYuANg/warden/controller/internal/mdns"
	"github.com/DaiYuANg/warden/controller/internal/raft"
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
