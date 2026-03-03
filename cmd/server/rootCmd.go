package main

import (
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var commands = []*cobra.Command{
	tokenCmd,
	workloadCmd,
	serverCmd,
	infoCmd,
}

var rootCmd = cobra.Command{
	Use: "cmd",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var confFiles []string

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	lo.ForEach(commands, func(item *cobra.Command, _ int) {
		rootCmd.AddCommand(item)
	})
	rootCmd.PersistentFlags().StringSliceVar(
		&confFiles,
		"conf",
		nil,
		"path to one or more config files (.yaml/.yml/.toml/.json), later files override earlier ones",
	)
}
