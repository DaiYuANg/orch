package cmd

import (
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var commands = []*cobra.Command{
	tokenCmd,
	serviceCmd,
	serverCmd,
	infoCmd,
}

var rootCmd = cobra.Command{
	Use: "cmd",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	lo.ForEach(commands, func(item *cobra.Command, _ int) {
		rootCmd.AddCommand(item)
	})
	f := flag.NewFlagSet("config", flag.ContinueOnError)
	f.StringSlice("conf", []string{"mock/mock.toml"}, "path to one or more .toml config files")
	f.String("time", "2020-01-01", "a time string")
	f.String("type", "xxx", "type of the app")
}
