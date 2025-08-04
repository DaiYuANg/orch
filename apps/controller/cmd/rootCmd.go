package cmd

import (
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var commands = []*cobra.Command{
	tokenCmd,
	serviceCmd,
	serverCmd,
}

var rootCmd = cobra.Command{
	Use: "cmd",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	lo.ForEach(commands, func(item *cobra.Command, _ int) {
		rootCmd.AddCommand(item)
	})
}
