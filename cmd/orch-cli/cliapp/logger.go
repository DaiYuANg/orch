package cliapp

import (
	"log/slog"

	"github.com/arcgolabs/dix"
)

var quietLogger = slog.New(slog.DiscardHandler)

func moduleLogger() dix.Module {
	return dix.NewModule(
		"cli-logger",
		dix.Providers(
			dix.Value(quietLogger),
		),
	)
}
