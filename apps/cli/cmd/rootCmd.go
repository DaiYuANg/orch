package cmd

import "github.com/spf13/cobra"

var rootCmd = cobra.Command{
	Use: "cmd",
}

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(listCmd)
}
