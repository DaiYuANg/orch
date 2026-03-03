package mdns

import (
	"log/slog"

	"github.com/hashicorp/mdns"
	"go.uber.org/fx"
	"os"
)

var Module = fx.Module("mdns", fx.Provide(newMdns), fx.Invoke(lifecycle))

func newMdns() (*mdns.MDNSService, error) {
	host, _ := os.Hostname()
	info := []string{"warnden service"}
	return mdns.NewMDNSService(host, "_foobar._tcp", "", "", 8000, nil, info)
}

func lifecycle(lc fx.Lifecycle, service *mdns.MDNSService, logger *slog.Logger) {
	lc.Append(fx.StartHook(func() {
		server, _ := mdns.NewServer(&mdns.Config{Zone: service})
		defer func(server *mdns.Server) {
			err := server.Shutdown()
			if err != nil {
				logger.Error("mdns server shutdown error", "error", err)
			}
		}(server)
	}))
}
