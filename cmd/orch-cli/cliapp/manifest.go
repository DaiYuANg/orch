package cliapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/deploy/orch"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// ManifestEnv is reserved for manifest-local dependencies (paths, schema validators, etc.).
type ManifestEnv struct{}

func moduleManifest() dix.Module {
	return dix.NewModule(
		"manifest",
		dix.Providers(
			dix.Provider0(func() ManifestEnv { return ManifestEnv{} }),
		),
	)
}

func NewManifestApp() *dix.App {
	return dix.New(
		"orch-cli-manifest",
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom0(slog.Default),
		dix.WithModules(
			moduleManifest(),
			orch.Module(),
			loader.Module(),
		),
	)
}

// RunManifest starts a manifest-scoped graph (deploy/orch + deploy/loader) and runs fn.
func RunManifest(ctx context.Context, fn func(ctx context.Context, lg *slog.Logger, deploy *loader.Loader) error) error {
	app := NewManifestApp()
	rt, err := app.Start(ctx)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "start orch-cli-manifest")
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() {
		if stopErr := rt.Stop(stopCtx); stopErr != nil {
			rt.Logger().Warn("runtime stop", "error", stopErr)
		}
	}()

	deploy, err := dix.ResolveAs[*loader.Loader](rt.Container())
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "resolve deploy loader")
	}
	return fn(ctx, rt.Logger(), deploy)
}
