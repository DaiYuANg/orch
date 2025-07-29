package cmd

import "github.com/spf13/cobra"

var rootCmd = cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		container().Run()
	},
}

func Execute() error {
	return rootCmd.Execute()
}
