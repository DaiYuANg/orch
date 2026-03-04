package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available packs",
	Run: func(cmd *cobra.Command, args []string) {
		printPackTable(cmd, builtinCatalog)
	},
}

func printPackTable(cmd *cobra.Command, items []packInfo) {
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no packs found")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tRUNTIME\tDESCRIPTION")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Name, item.Version, item.Runtime, item.Description)
	}
	_ = w.Flush()
}
