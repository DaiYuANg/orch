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

var modules = []fx.Option{
	config.Module,
	auth.Module,
	mdns.Module,
	raft.Module,
	common.Module,
	endpoint.Module,
	http.Module,
	dns.Module,
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		app, err = injector.CreateContainer(
			config.Module,
			auth.Module,
			mdns.Module,
			raft.Module,
			common.Module,
			endpoint.Module,
			http.Module,
			dns.Module,
		)
		if err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		app.Run()
	},
}
