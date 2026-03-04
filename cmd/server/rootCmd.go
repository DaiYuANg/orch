package main

import (
	"time"

	"github.com/spf13/cobra"
)

var commands = []*cobra.Command{
	tokenCmd,
	workloadCmd,
	serverCmd,
	infoCmd,
}

var rootCmd = cobra.Command{
	Use:   "warden",
	Short: "Warden control plane CLI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var confFiles []string
var apiAddress string
var authToken string
var authTokenFile string
var requestTimeout time.Duration

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	for _, item := range commands {
		rootCmd.AddCommand(item)
	}
	rootCmd.PersistentFlags().StringSliceVar(
		&confFiles,
		"conf",
		nil,
		"path to one or more config files (.yaml/.yml/.toml/.json), later files override earlier ones",
	)
	rootCmd.PersistentFlags().StringVar(
		&apiAddress,
		"api",
		"http://127.0.0.1:7443",
		"warden http api base url",
	)
	rootCmd.PersistentFlags().StringVar(
		&authToken,
		"token",
		"",
		"bearer token for warden api",
	)
	rootCmd.PersistentFlags().StringVar(
		&authTokenFile,
		"token-file",
		"",
		"path to file containing bearer token (default: temp/warden.token if exists)",
	)
	rootCmd.PersistentFlags().DurationVar(
		&requestTimeout,
		"timeout",
		10*time.Second,
		"http request timeout",
	)
}
