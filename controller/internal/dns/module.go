package dns

import (
	"github.com/miekg/dns"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("dns", fx.Provide(newDns), fx.Invoke(lifecycle))

func newDns() *dns.Server {
	return &dns.Server{Addr: ":53", Net: "udp"}
}

func lifecycle(lc fx.Lifecycle, server *dns.Server, log *zap.SugaredLogger) {
	lc.Append(fx.StartHook(func() {
		go func() {
			err := lo.Must1(server.ListenAndServe(), "dns server")
			if err != nil {
				log.Errorw("dns server error", "error", err)
			}
		}()
	}))
}
