package main

import (
	"github.com/DaiYuANg/warden/internal/auth"
	"github.com/DaiYuANg/warden/internal/common"
	"github.com/DaiYuANg/warden/internal/config"
	"github.com/DaiYuANg/warden/internal/dns"
	"github.com/DaiYuANg/warden/internal/endpoint"
	"github.com/DaiYuANg/warden/internal/http"
	"github.com/DaiYuANg/warden/internal/injector"
	"github.com/DaiYuANg/warden/internal/mdns"
	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var app *fx.App

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	PreRun: func(cmd *cobra.Command, args []string) {
		app = injector.CreateContainer(
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
