package main

import (
	"context"

	"github.com/arcgolabs/dix"
	"github.com/pterm/pterm"
)

func warnRuntimeStop(ctx context.Context, rt *dix.Runtime, label string) {
	report, stopErr := rt.StopWithReport(ctx)
	if stopErr != nil {
		pterm.Warning.Printfln("%s: %v", label, stopErr)
		return
	}
	if report != nil && report.HasErrors() {
		pterm.Warning.Printfln("%s: %v", label, report.Err())
	}
}
