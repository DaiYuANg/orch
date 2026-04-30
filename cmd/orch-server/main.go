package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pterm/pterm"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	runErr := newRootCmd().ExecuteContext(ctx)
	if runErr != nil {
		pterm.Error.Println(runErr)
	}
	cancel()
	if runErr != nil {
		os.Exit(1)
	}
}
