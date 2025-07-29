package pkg

import (
	"crypto/sha1"
	"fmt"
	"net"
	"os"
)

func GenerateNodeID() string {
	hostname, _ := os.Hostname()
	_, _ = net.InterfaceAddrs()
	mac := ""
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) > 0 {
			mac = iface.HardwareAddr.String()
			break
		}
	}
	hash := sha1.Sum([]byte(hostname + mac))
	return fmt.Sprintf("node-%x", hash[:6])
}
