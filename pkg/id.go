package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/net"
)

func MachineID() (string, error) {
	var parts []string
	// 主机信息
	hInfo, err := host.Info()
	if err == nil {
		parts = lo.Filter([]string{hInfo.Hostname, hInfo.OS, hInfo.Platform}, func(item string, _ int) bool {
			return strings.TrimSpace(item) != ""
		})
	}

	// MAC 地址
	ifaces, err := net.Interfaces()
	if err == nil {
		hardwareAddrs := lo.FilterMap(ifaces, func(iface net.InterfaceStat, _ int) (string, bool) {
			if len(iface.HardwareAddr) == 0 {
				return "", false
			}
			return iface.HardwareAddr, true
		})
		parts = append(parts, hardwareAddrs...)
	}

	mid := sysMachineId()
	parts = append(parts, mid)

	data := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:]), nil
}
