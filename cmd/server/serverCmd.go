package main

import (
	"github.com/DaiYuANg/warden/internal/auth"
	"github.com/DaiYuANg/warden/internal/common"
	"github.com/DaiYuANg/warden/internal/config"
	"github.com/DaiYuANg/warden/internal/dns"
	"github.com/DaiYuANg/warden/internal/endpoint"
	"github.com/DaiYuANg/warden/internal/http"
	"github.com/DaiYuANg/warden/internal/ingress"
	"github.com/DaiYuANg/warden/internal/injector"
	"github.com/DaiYuANg/warden/internal/mdns"
	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/DaiYuANg/warden/internal/task"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var app *fx.App

var modules = []fx.Option{
	config.Module,
	auth.Module,
	mdns.Module,
	raft.Module,
	registry.Module,
	common.Module,
	task.Module,
	endpoint.Module,
	http.Module,
	ingress.Module,
	dns.Module,
}

var runCmd = &cobra.Command{
	Use:     "run",
	Aliases: []string{"server", "start"},
	Short:   "Run warden control plane server",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		options := append(
			[]fx.Option{
				fx.Supply(fx.Annotate(confFiles, fx.ResultTags(`name:"conf"`))),
			},
			modules...,
		)
		app, err = injector.CreateContainer(options...)
		if err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		app.Run()
	},
}
