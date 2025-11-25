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
		if hInfo.Hostname != "" {
			parts = append(parts, hInfo.Hostname)
		}
		if hInfo.OS != "" {
			parts = append(parts, hInfo.OS)
		}
		if hInfo.Platform != "" {
			parts = append(parts, hInfo.Platform)
		}
	}

	// MAC 地址
	ifaces, err := net.Interfaces()
	if err == nil {
		lo.ForEach(ifaces, func(iface net.InterfaceStat, index int) {
			if len(iface.HardwareAddr) > 0 {
				parts = append(parts, iface.HardwareAddr)
			}
		})
	}

	mid := sysMachineId()
	parts = append(parts, mid)

	data := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:]), nil
}
