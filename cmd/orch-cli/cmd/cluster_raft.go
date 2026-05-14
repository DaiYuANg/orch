package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/cmd/orch-cli/cliapp"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newRaftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raft",
		Short: "Inspect and manage Raft membership",
		Long:  `Raft membership commands operate on the configured control plane. Followers forward writes to the known leader when cluster.nodes maps the leader ID to an API URL.`,
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newRaftStatusCmd())
	cmd.AddCommand(newRaftMembersCmd())
	cmd.AddCommand(newRaftAddVoterCmd())
	cmd.AddCommand(newRaftRemoveVoterCmd())
	return cmd
}

func newRaftStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local Raft state and leader identity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.RaftStatus(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "raft status")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeRaftStatusHuman(out)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRaftMembersCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "members",
		Aliases: []string{"member", "peers"},
		Short:   "List Raft members",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.ListRaftMembers(ctx)
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "list raft members")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body.Items)
				}
				return writeRaftMembersHuman(out.Body.Items)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON array")
	return cmd
}

func newRaftAddVoterCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "add-voter ID ADDRESS",
		Short: "Add or update a Raft voter",
		Long:  `Adds or updates a Raft voter by node ID and advertised raft host:port. Followers forward writes when configured with the leader API URL.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.AddRaftVoter(ctx, args[0], args[1])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "add raft voter")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("raft",
					viewField("status", statusBadge("accepted")),
					viewField("id", out.Body.Member.ID),
					viewField("address", out.Body.Member.Address),
				)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}

func newRaftRemoveVoterCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "remove-voter ID",
		Aliases: []string{"remove-member"},
		Short:   "Remove a Raft member",
		Long:    `Removes a Raft server from membership. Followers forward writes when configured with the leader API URL.`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := contextFromCmd(cmd)
			conn := cliapp.ConnFromGlobals(serverURL, authToken)
			return cliapp.RunCluster(ctx, conn, func(ctx context.Context, c *apiclient.Client, _ *loader.Loader) error {
				out, err := c.RemoveRaftMember(ctx, args[0])
				if err != nil {
					return oopsx.B("cli").Wrapf(err, "remove raft member")
				}
				if jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out.Body)
				}
				return writeInfoLine("raft",
					viewField("status", statusBadge("accepted")),
					viewField("id", out.Body.ID),
				)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print JSON")
	return cmd
}
