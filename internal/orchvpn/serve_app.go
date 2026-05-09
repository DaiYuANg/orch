package orchvpn

import (
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
		dix.Modules(
			buildmeta.Module(),
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
			dix.Value(svcCfg),
			dix.Provider2(NewServerDaemonService),
		),
	)
}
