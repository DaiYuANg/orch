package main

import (
	"github.com/DaiYuANg/warden/pack/cmd"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(cmd.Execute())
}
