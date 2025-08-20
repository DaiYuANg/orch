package main

import "github.com/spf13/cobra"

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get information about the resource",
	Run: func(cmd *cobra.Command, args []string) {

	},
}
