package main

import "github.com/spf13/cobra"

var unitCmd = &cobra.Command{
	Use:    "unit",
	Short:  "Legacy alias of service list",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceList(cmd, args)
	},
}

func init() {
	unitCmd.AddCommand(taskCmd)
}
