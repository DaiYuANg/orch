package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "Run a unit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("unit called")
	},
}
