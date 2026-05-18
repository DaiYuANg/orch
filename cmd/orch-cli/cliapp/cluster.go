package cliapp

import (
	"context"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/pterm/pterm"

	"github.com/lyonbrown4d/orch/internal/apiclient"
	"github.com/lyonbrown4d/orch/internal/buildmeta"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/deploy/orch"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// Conn holds values from global CLI flags (orch --server / --token); it is the injectable boundary for cluster commands.
type Conn struct {
	ServerURL string
	Token     string
}

// ConnFromGlobals builds Conn from orch persistent flags (parsed before RunE).
func ConnFromGlobals(serverURL, token string) Conn {
	return Conn{ServerURL: serverURL, Token: token}
}

// NewClusterApp wires the HTTP client + lifecycle (close on Stop) for commands that talk to orch-server.
func NewClusterApp(conn Conn) *dix.App {
	return dix.New(
		"orch-cli-cluster",
		dix.Modules(
			buildmeta.Module(),
			moduleLogger(),
			moduleConn(conn),
			moduleClusterClient(),
			orch.Module(),
			loader.Module(),
		),
	)
}

func moduleConn(c Conn) dix.Module {
	return dix.NewModule(
		"cli-conn",
		dix.Providers(
			dix.Value(c),
		),
	)
}

func moduleClusterClient() dix.Module {
	return dix.NewModule(
		"cluster-client",
		dix.Providers(
			dix.ProviderErr1(func(conn Conn) (*apiclient.Client, error) {
				return apiclient.New(conn.ServerURL, conn.Token)
			}),
		),
		dix.Hooks(
			dix.OnStop(func(ctx context.Context, c *apiclient.Client) error {
				if c == nil {
					return nil
				}
				return c.Close()
			}),
		),
	)
}

// RunCluster builds a short-lived app, Starts it, resolves *apiclient.Client and [loader.Loader], runs fn, then Stops.
func RunCluster(ctx context.Context, conn Conn, fn func(ctx context.Context, c *apiclient.Client, deploy *loader.Loader) error) error {
	app := NewClusterApp(conn)
	rt, err := app.Start(ctx)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "start orch-cli-cluster")
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() {
		report, stopErr := rt.StopWithReport(stopCtx)
		if stopErr != nil {
			pterm.Warning.Printfln("orch-cli cluster runtime stop: %v", stopErr)
			return
		}
		if report != nil && report.HasErrors() {
			pterm.Warning.Printfln("orch-cli cluster runtime stop: %v", report.Err())
		}
	}()

	c, err := dix.ResolveAsContext[*apiclient.Client](ctx, rt.Container())
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "resolve HTTP client")
	}
	deploy, err := dix.ResolveAsContext[*loader.Loader](ctx, rt.Container())
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "resolve deploy loader")
	}
	return fn(ctx, c, deploy)
}
