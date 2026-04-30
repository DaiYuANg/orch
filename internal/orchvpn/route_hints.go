package orchvpn

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

func printRouteHints(log *slog.Logger, ifName string, cidrs []string) {
	if log == nil {
		log = slog.Default()
	}
	log = log.With(slog.String("component", "orchvpn-routes"))
	if len(cidrs) == 0 {
		log.Debug("no container_routes from bootstrap; assign a TUN address and routes manually or extend the API")
		return
	}
	routeList := strings.Join(cidrs, ", ")
	switch runtime.GOOS {
	case "windows":
		log.Info("manual route setup (Windows, elevated)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd",
			fmt.Sprintf(`netsh interface ip set address name=%q static <addr> <mask>`, ifName))
		for _, c := range cidrs {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`route add %s <gateway_or_onlink> IF <ifIndex>`, c))
		}
		log.Debug("route hint note", "detail", "use Get-NetAdapter / route print for ifIndex; orch-vpn does not add routes yet")
	case "darwin":
		log.Info("manual route setup (macOS)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ifconfig %s <addr> <peer> mtu <mtu> up`, ifName))
		for _, c := range cidrs {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo route add -net %s -interface %s`, c, ifName))
		}
	default:
		log.Info("manual route setup (Linux)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ip addr add <addr>/32 dev %s`, ifName))
		log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ip link set %s up`, ifName))
		for _, c := range cidrs {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ip route add %s dev %s`, c, ifName))
		}
	}
}
