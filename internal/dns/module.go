package dns

import (
	"log/slog"

	"go.uber.org/fx"
)

var Module = fx.Module("dns", fx.Provide(newDns), fx.Invoke(lifecycle))

func newDns(logger *slog.Logger) (*DNSServer, error) {
	return NewDNSServer(logger)
}

func lifecycle(lc fx.Lifecycle, server *DNSServer) {
	lc.Append(fx.StartHook(func() {
		go func() {
			server.Serve(":53")
		}()
	}))
}
