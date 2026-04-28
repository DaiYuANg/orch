package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		reportExitError(err)
		os.Exit(1)
	}
}
