package cmd

import (
	"github.com/spf13/cobra"
)

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
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(serverCmd)
}
