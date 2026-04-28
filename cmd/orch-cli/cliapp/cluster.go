package cliapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
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
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom0(slog.Default),
		dix.WithModules(
			moduleConn(conn),
			moduleClusterClient(),
		),
	)
}

func moduleConn(c Conn) dix.Module {
	return dix.NewModule(
		"cli-conn",
		dix.Providers(
			dix.Provider0(func() Conn { return c }),
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

// RunCluster builds a short-lived app, Starts it, resolves *apiclient.Client, runs fn, then Stops (invokes Close via OnStop).
func RunCluster(ctx context.Context, conn Conn, fn func(ctx context.Context, c *apiclient.Client) error) error {
	app := NewClusterApp(conn)
	rt, err := app.Start(ctx)
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "start orch-cli-cluster")
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() {
		if stopErr := rt.Stop(stopCtx); stopErr != nil {
			rt.Logger().Warn("runtime stop", "error", stopErr)
		}
	}()

	c, err := dix.ResolveAs[*apiclient.Client](rt.Container())
	if err != nil {
		return oopsx.B("cli").Wrapf(err, "resolve HTTP client")
	}
	return fn(ctx, c)
}
