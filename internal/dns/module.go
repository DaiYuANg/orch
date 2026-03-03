package dns

import (
	"log/slog"
	"strings"

	"github.com/DaiYuANg/warden/internal/config"
	"github.com/DaiYuANg/warden/internal/registry"
	"go.uber.org/fx"
)

var Module = fx.Module("dns", fx.Provide(newDNS), fx.Invoke(lifecycle))

type newDNSDependency struct {
	fx.In
	Config   *config.Config
	Logger   *slog.Logger
	Registry *registry.Service
}

func newDNS(dep newDNSDependency) *DNSServer {
	return NewDNSServer(dep.Logger, dep.Registry)
}

func lifecycle(lc fx.Lifecycle, cfg *config.Config, server *DNSServer) {
	addr := strings.TrimSpace(cfg.Network.DNSListen)
	if addr == "" {
		addr = ":1053"
	}
	lc.Append(fx.StartStopHook(
		func() error {
			return server.Serve(addr)
		},
		func() error {
			return server.Shutdown()
		},
	))
}
