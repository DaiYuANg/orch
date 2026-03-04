package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var tokenPathOnly bool

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print local API bearer token or token file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, source, err := resolveToken()
		if err != nil {
			return err
		}
		if tokenPathOnly {
			if source == "" {
				return errors.New("token file was not found; provide --token-file")
			}
			fmt.Fprintln(cmd.OutOrStdout(), source)
			return nil
		}
		if token == "" {
			return errors.New("token not found; provide --token or --token-file")
		}
		fmt.Fprintln(cmd.OutOrStdout(), token)
		return nil
	},
}

func init() {
	tokenCmd.Flags().BoolVar(&tokenPathOnly, "path", false, "print token source path instead of token value")
}
