package main

import "github.com/spf13/cobra"

var rootCmd = cobra.Command{Use: "pack"}

func Execute() error {
	return rootCmd.Execute()
}
