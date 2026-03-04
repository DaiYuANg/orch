package main

import "github.com/spf13/cobra"

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get system information from warden API",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		payload := map[string]any{}
		if err := client.Get("/system/info", &payload); err != nil {
			return err
		}
		return printJSON(payload)
	},
}
