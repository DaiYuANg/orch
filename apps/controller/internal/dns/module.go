package dns

import (
	"github.com/DaiYuANg/warden/dns"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("dns", fx.Provide(newDns), fx.Invoke(lifecycle))

func newDns(logger *zap.SugaredLogger) *dns.DNSServer {
	return dns.NewDNSServer(logger)
}

func lifecycle(lc fx.Lifecycle, server *dns.DNSServer) {
	lc.Append(fx.StartHook(func() {
		go func() {
			server.Serve(":53")
		}()
	}))
}
