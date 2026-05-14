package cliapp

import (
	"context"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/pterm/pterm"

	"github.com/lyonbrown4d/orch/internal/buildmeta"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/deploy/orch"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// ManifestEnv is reserved for manifest-local dependencies (paths, schema validators, etc.).
type ManifestEnv struct{}

func moduleManifest() dix.Module {
	return dix.NewModule(
		"manifest",
		dix.Providers(
			dix.Value(ManifestEnv{}),
		),
	)
}

func NewManifestApp() *dix.App {
	return dix.New(
		"orch-cli-manifest",
		dix.Modules(
			buildmeta.Module(),
			moduleLogger(),
			moduleManifest(),
			orch.Module(),
			loader.Module(),
		),
	)
}

// RunManifest starts a manifest-scoped graph (deploy/orch + deploy/loader) and runs fn.
func RunManifest(ctx context.Context, fn func(ctx context.Context, deploy *loader.Loader) error) error {
	app := NewManifestApp()
	rt, err := app.Start(ctx)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "start orch-cli-manifest")
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() {
		if stopErr := rt.Stop(stopCtx); stopErr != nil {
			pterm.Warning.Printfln("orch-cli manifest runtime stop: %v", stopErr)
		}
	}()

	deploy, err := dix.ResolveAsContext[*loader.Loader](ctx, rt.Container())
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "resolve deploy loader")
	}
	return fn(ctx, deploy)
}
