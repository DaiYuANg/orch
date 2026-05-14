package orchvpn

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/arcgolabs/collectionx/list"
)

func printRouteHints(log *slog.Logger, ifName string, cidrs *list.List[string]) {
	if log == nil {
		log = slog.Default()
	}
	log = log.With(slog.String("component", "orchvpn-routes"))
	if cidrs.Len() == 0 {
		log.Debug("no container_routes from bootstrap; assign a TUN address and routes manually or extend the API")
		return
	}
	routeList := cidrs.Join(", ")
	switch runtime.GOOS {
	case "windows":
		log.Info("manual route setup (Windows, elevated)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd",
			fmt.Sprintf(`netsh interface ip set address name=%q static <addr> <mask>`, ifName))
		cidrs.Range(func(_ int, c string) bool {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`route add %s <gateway_or_onlink> IF <ifIndex>`, c))
			return true
		})
		log.Debug("route hint note", "detail", "use Get-NetAdapter / route print for ifIndex; orch-vpn does not add routes yet")
	case "darwin":
		log.Info("manual route setup (macOS)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ifconfig %s <addr> <peer> mtu <mtu> up`, ifName))
		cidrs.Range(func(_ int, c string) bool {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo route add -net %s -interface %s`, c, ifName))
			return true
		})
	default:
		log.Info("manual route setup (Linux)", "if_name", ifName, "routes", routeList)
		log.Debug("route hint example", "cmd", "sudo ip addr add <addr>/32 dev "+ifName)
		log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ip link set %s up`, ifName))
		cidrs.Range(func(_ int, c string) bool {
			log.Debug("route hint example", "cmd", fmt.Sprintf(`sudo ip route add %s dev %s`, c, ifName))
			return true
		})
	}
}
