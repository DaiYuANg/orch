package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for a pack",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("search called")
	},
}
