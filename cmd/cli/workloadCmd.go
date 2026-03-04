package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	tasksvc "github.com/DaiYuANg/warden/internal/task"
	"github.com/spf13/cobra"
)

var workloadCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage deployed services",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceList(cmd, args)
	},
}

func init() {
	workloadCmd.AddCommand(
		serviceListCmd,
		serviceGetCmd,
		serviceDeployCmd,
		serviceStopCmd,
		serviceLogsCmd,
		serviceMigrateCmd,
		serviceFailoverCmd,
		serviceRebalanceCmd,
		unitCmd,
	)
	serviceDeployCmd.Flags().StringVarP(&deployFile, "file", "f", "", "dsl file path (.yaml/.yml/.hcl)")
	serviceDeployCmd.Flags().StringVar(&deployContent, "content", "", "dsl content")
	serviceDeployCmd.Flags().StringVar(&deployFormat, "format", "", "dsl format override: yaml|hcl")
	serviceLogsCmd.Flags().IntVar(&serviceLogsTail, "tail", 200, "number of log lines")
	serviceMigrateCmd.Flags().StringVar(&serviceMigrateTargetNode, "target-node", "", "target worker node id")
	serviceMigrateCmd.Flags().BoolVar(&serviceMigrateForceStateful, "force-stateful", false, "allow migrating stateful workloads")
	serviceMigrateCmd.Flags().IntVar(&serviceMigrateMaxUnavailable, "max-unavailable", 1, "max unavailable instances during stateful migration")
	serviceFailoverCmd.Flags().StringVar(&serviceFailoverFailedNode, "failed-node", "", "failed worker node id")
	serviceFailoverCmd.Flags().StringVar(&serviceFailoverTargetNode, "target-node", "", "optional explicit failover target node id")
	serviceFailoverCmd.Flags().BoolVar(&serviceFailoverForceStateful, "force-stateful", false, "allow failover for stateful workloads")
	serviceFailoverCmd.Flags().IntVar(&serviceFailoverMaxUnavailable, "max-unavailable", 1, "max unavailable instances during stateful failover")
	serviceRebalanceCmd.Flags().IntVar(&serviceRebalanceMaxMigrations, "max-migrations", 0, "max migrations in one rebalance run (0 means auto)")
	serviceRebalanceCmd.Flags().BoolVar(&serviceRebalanceForceStateful, "force-stateful", false, "allow rebalancing stateful workloads")
	serviceRebalanceCmd.Flags().IntVar(&serviceRebalanceMaxUnavailable, "max-unavailable", 1, "max unavailable instances during stateful rebalance")
}

var deployFile string
var deployContent string
var deployFormat string
var serviceLogsTail int
var serviceMigrateTargetNode string
var serviceMigrateForceStateful bool
var serviceMigrateMaxUnavailable int
var serviceFailoverFailedNode string
var serviceFailoverTargetNode string
var serviceFailoverForceStateful bool
var serviceFailoverMaxUnavailable int
var serviceRebalanceMaxMigrations int
var serviceRebalanceForceStateful bool
var serviceRebalanceMaxUnavailable int

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	RunE:  runServiceList,
}

var serviceGetCmd = &cobra.Command{
	Use:   "get <deployment-id>",
	Short: "Get deployment detail",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		var detail tasksvc.DeploymentDetail
		if err := client.Get("/tasks/"+url.PathEscape(args[0]), &detail); err != nil {
			return err
		}
		return printJSON(detail)
	},
}

var serviceDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a service workload from DSL",
	RunE: func(cmd *cobra.Command, args []string) error {
		content := strings.TrimSpace(deployContent)
		filename := strings.TrimSpace(deployFile)
		if filename != "" {
			data, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			content = string(data)
			filename = filepath.Base(filename)
		}

		if content == "" {
			return errors.New("either --file or --content is required")
		}

		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		req := struct {
			Filename string `json:"filename,omitempty"`
			Format   string `json:"format,omitempty"`
			Content  string `json:"content"`
		}{
			Filename: filename,
			Format:   strings.TrimSpace(deployFormat),
			Content:  content,
		}

		var result tasksvc.DeployResult
		if err := client.Post("/tasks/deploy", req, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop <deployment-id>",
	Short: "Stop a deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		var stopped struct {
			Stopped bool `json:"stopped"`
		}
		if err := client.Post("/tasks/"+url.PathEscape(args[0])+"/stop", map[string]any{}, &stopped); err != nil {
			return err
		}
		return printJSON(stopped)
	},
}

var serviceLogsCmd = &cobra.Command{
	Use:   "logs <instance-id>",
	Short: "Get logs for a running instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		var payload struct {
			Logs string `json:"logs"`
		}
		path := buildURLPath("/tasks/instances/"+url.PathEscape(args[0])+"/logs", map[string]string{
			"tail": fmt.Sprintf("%d", serviceLogsTail),
		})
		if err := client.Get(path, &payload); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), payload.Logs)
		return nil
	},
}

var serviceMigrateCmd = &cobra.Command{
	Use:   "migrate <deployment-id>",
	Short: "Migrate deployment instances to another worker node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		req := tasksvc.MigrateDeploymentRequest{
			TargetNode:     strings.TrimSpace(serviceMigrateTargetNode),
			ForceStateful:  serviceMigrateForceStateful,
			MaxUnavailable: serviceMigrateMaxUnavailable,
		}
		var result tasksvc.MigrateDeploymentResult
		if err := client.Post("/tasks/"+url.PathEscape(args[0])+"/migrate", req, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

var serviceFailoverCmd = &cobra.Command{
	Use:   "failover",
	Short: "Fail over deployments from a failed worker node",
	RunE: func(cmd *cobra.Command, args []string) error {
		failedNode := strings.TrimSpace(serviceFailoverFailedNode)
		if failedNode == "" {
			return errors.New("--failed-node is required")
		}

		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		req := tasksvc.FailoverRequest{
			FailedNode:     failedNode,
			TargetNode:     strings.TrimSpace(serviceFailoverTargetNode),
			ForceStateful:  serviceFailoverForceStateful,
			MaxUnavailable: serviceFailoverMaxUnavailable,
		}
		var result tasksvc.FailoverResult
		if err := client.Post("/tasks/failover", req, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

var serviceRebalanceCmd = &cobra.Command{
	Use:   "rebalance",
	Short: "Rebalance running deployments across worker nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newDefaultAPIClient()
		if err != nil {
			return err
		}

		req := tasksvc.RebalanceRequest{
			MaxMigrations:  serviceRebalanceMaxMigrations,
			ForceStateful:  serviceRebalanceForceStateful,
			MaxUnavailable: serviceRebalanceMaxUnavailable,
		}
		var result tasksvc.RebalanceResult
		if err := client.Post("/tasks/rebalance", req, &result); err != nil {
			return err
		}
		return printJSON(result)
	},
}

func runServiceList(cmd *cobra.Command, args []string) error {
	client, err := newDefaultAPIClient()
	if err != nil {
		return err
	}

	var items []tasksvc.DeploymentInfo
	if err := client.Get("/tasks", &items); err != nil {
		return err
	}
	return printJSON(items)
}
