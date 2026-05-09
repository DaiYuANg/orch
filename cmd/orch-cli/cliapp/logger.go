package cliapp

import (
	"io"
	"log/slog"

	"github.com/arcgolabs/dix"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func moduleLogger() dix.Module {
	return dix.NewModule(
		"cli-logger",
		dix.Providers(
			dix.Value(quietLogger),
		),
	)
}
