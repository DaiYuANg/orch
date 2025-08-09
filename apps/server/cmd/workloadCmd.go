package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var workloadCmd = &cobra.Command{
	Use:   "service",
	Short: "service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("service called")
	},
}

func init() {
	workloadCmd.AddCommand(unitCmd)
}
