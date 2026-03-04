package main

import (
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search packs by name or description",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyword := strings.ToLower(strings.TrimSpace(args[0]))
		filtered := make([]packInfo, 0)
		for _, item := range builtinCatalog {
			if strings.Contains(strings.ToLower(item.Name), keyword) ||
				strings.Contains(strings.ToLower(item.Description), keyword) {
				filtered = append(filtered, item)
			}
		}
		printPackTable(cmd, filtered)
	},
}
