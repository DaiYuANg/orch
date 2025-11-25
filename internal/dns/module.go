package dns

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("dns", fx.Provide(newDns), fx.Invoke(lifecycle))

func newDns(logger *zap.SugaredLogger) (*DNSServer, error) {
	return NewDNSServer(logger)
}

func lifecycle(lc fx.Lifecycle, server *DNSServer) {
	lc.Append(fx.StartHook(func() {
		go func() {
			server.Serve(":53")
		}()
	}))
}
