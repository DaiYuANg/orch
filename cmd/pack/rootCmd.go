package main

import "github.com/spf13/cobra"

var rootCmd = cobra.Command{
	Use:   "pack",
	Short: "Manage Warden packs",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(listCmd, searchCmd)
}
