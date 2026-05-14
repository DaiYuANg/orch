package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pterm/pterm"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		pterm.Error.Println(err)
		return 1
	}
	return 0
}
