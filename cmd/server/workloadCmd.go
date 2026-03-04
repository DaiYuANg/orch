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
	workloadCmd.AddCommand(serviceListCmd, serviceGetCmd, serviceDeployCmd, serviceStopCmd, serviceLogsCmd, unitCmd)
	serviceDeployCmd.Flags().StringVarP(&deployFile, "file", "f", "", "dsl file path (.yaml/.yml/.hcl)")
	serviceDeployCmd.Flags().StringVar(&deployContent, "content", "", "dsl content")
	serviceDeployCmd.Flags().StringVar(&deployFormat, "format", "", "dsl format override: yaml|hcl")
	serviceLogsCmd.Flags().IntVar(&serviceLogsTail, "tail", 200, "number of log lines")
}

var deployFile string
var deployContent string
var deployFormat string
var serviceLogsTail int

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
