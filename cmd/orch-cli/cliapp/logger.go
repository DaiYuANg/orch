package cliapp

import (
	"io"
	"log/slog"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func logger() *slog.Logger {
	return quietLogger
}
