package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "wctl",
	Short: "wctl",
	Long:  `wctl`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}
