package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = cobra.Command{
	Use:   "warden-server",
	Short: "Warden control plane server process",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var confFiles []string

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.PersistentFlags().StringSliceVar(
		&confFiles,
		"conf",
		nil,
		"path to one or more config files (.yaml/.yml/.toml/.json), later files override earlier ones",
	)
}
