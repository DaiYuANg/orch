package main

import (
	"github.com/DaiYuANg/warden/server/cmd"
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(cmd.Execute())
}
