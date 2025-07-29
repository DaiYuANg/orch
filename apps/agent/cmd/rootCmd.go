package cmd

import (
	"github.com/DaiYuANg/warden/agent/internal/schedule"
	"github.com/DaiYuANg/warden/container"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var app *fx.App

var rootCmd = cobra.Command{
	PreRun: func(cmd *cobra.Command, args []string) {
		app = container.CreateContainer(
			schedule.Module,
		)
	},
	Run: func(cmd *cobra.Command, args []string) {
		app.Run()
	},
}

func Execute() error {
	return rootCmd.Execute()
}
