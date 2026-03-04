package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

var clusterJoinID string
var clusterJoinAddress string
var clusterRemoveID string

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage raft cluster members and status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClusterStatus(cmd, args)
	},
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show raft cluster status",
	RunE:  runClusterStatus,
}

var clusterJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Add a raft voter member (leader only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(clusterJoinID)
		address := strings.TrimSpace(clusterJoinAddress)
		if id == "" {
			return errors.New("--id is required")
		}
		if address == "" {
			return errors.New("--address is required")
		}

		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		var result map[string]any
		if err := client.Post("/system/cluster/join", map[string]string{
			"id":      id,
			"address": address,
		}, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

var clusterRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a raft member (leader only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(clusterRemoveID)
		if id == "" {
			return errors.New("--id is required")
		}

		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		var result map[string]any
		if err := client.Post("/system/cluster/remove", map[string]string{
			"id": id,
		}, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

func runClusterStatus(cmd *cobra.Command, args []string) error {
	client, err := newDefaultAPIClient()
	if err != nil {
		return err
	}

	payload := map[string]any{}
	if err := client.Get("/system/cluster", &payload); err != nil {
		return err
	}
	return printJSON(payload)
}

func init() {
	clusterCmd.AddCommand(clusterStatusCmd, clusterJoinCmd, clusterRemoveCmd)

	clusterJoinCmd.Flags().StringVar(&clusterJoinID, "id", "", "raft server id")
	clusterJoinCmd.Flags().StringVar(&clusterJoinAddress, "address", "", "raft server address host:port")

	clusterRemoveCmd.Flags().StringVar(&clusterRemoveID, "id", "", "raft server id")
}
