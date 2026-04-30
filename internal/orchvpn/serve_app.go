package orchvpn

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
)

// NewServeApp builds the dix graph for `orch-vpn serve` (standalone UDP encap-v0 listener for dev).
// cfg is the shared orch config (log level, etc.) from config.Load / env ORCH_*.
func NewServeApp(svcCfg ServerConfig, cfg config.Config) *dix.App {
	return dix.New(
		"orch-vpn-serve",
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom1(func(logger *slog.Logger) *slog.Logger { return logger }),
		dix.WithModules(
			config.Static(cfg),
			logging.Module(),
			serveModule(svcCfg),
		),
	)
}

func serveModule(svcCfg ServerConfig) dix.Module {
	return dix.NewModule(
		"orch-vpn-serve",
		dix.Providers(
			dix.Provider0(func() ServerConfig { return svcCfg }),
			dix.Provider2(NewServerDaemonService),
		),
	)
}
