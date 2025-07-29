package cmd

import "github.com/spf13/cobra"

var rootCmd = cobra.Command{
	Use: "cmd",
	Run: func(cmd *cobra.Command, args []string) {
		container().Run()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
