package cliapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/buildmeta"
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

// NewManifestApp wires modules used only by validate/parse-style commands (no control-plane HTTP client).
func NewManifestApp() *dix.App {
	return dix.New(
		"orch-cli-manifest",
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom0(func() *slog.Logger { return slog.Default() }),
		dix.WithModules(
			moduleManifest(),
		),
	)
}

// RunManifest Starts a manifest-scoped graph and exposes the runtime logger for consistent observability hooks later.
func RunManifest(ctx context.Context, fn func(ctx context.Context, lg *slog.Logger) error) error {
	app := NewManifestApp()
	rt, err := app.Start(ctx)
	if err != nil {
		return err
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() { _ = rt.Stop(stopCtx) }()

	return fn(ctx, rt.Logger())
}
