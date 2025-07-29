package mdns

import (
	"github.com/hashicorp/mdns"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
)

var Module = fx.Module("mdns", fx.Provide(newMdns), fx.Invoke(lifecycle))

func newMdns() (*mdns.MDNSService, error) {
	host, _ := os.Hostname()
	info := []string{"My awesome service"}
	return mdns.NewMDNSService(host, "_foobar._tcp", "", "", 8000, nil, info)
}

func lifecycle(lc fx.Lifecycle, service *mdns.MDNSService, logger *zap.SugaredLogger) {
	lc.Append(fx.StartHook(func() {
		server, _ := mdns.NewServer(&mdns.Config{Zone: service})
		defer func(server *mdns.Server) {
			err := server.Shutdown()
			if err != nil {
				logger.Errorf("mdns server error%e", err)
			}
		}(server)
	}))
}
