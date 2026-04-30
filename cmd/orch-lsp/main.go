package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	runErr := newRootCmd().ExecuteContext(ctx)
	if runErr != nil {
		if _, werr := fmt.Fprintf(os.Stderr, "%v\n", runErr); werr != nil {
			os.Exit(2)
		}
	}
	cancel()
	if runErr != nil {
		os.Exit(1)
	}
}
