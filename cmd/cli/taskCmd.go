package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:    "task",
	Short:  "Legacy command, use 'service' subcommands",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("task command is deprecated, use: service list|get|deploy|stop|logs")
	},
}
