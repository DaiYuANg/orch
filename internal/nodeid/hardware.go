package nodeid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/host"
)

// FromHardware returns a stable identifier for this machine: prefer the OS host id
// (e.g. DMI product UUID, /etc/machine-id on Linux, MachineGuid on Windows), else a
// deterministic fingerprint from hostname + hardware MAC addresses.
func FromHardware(ctx context.Context) (string, error) {
	raw, err := host.HostIDWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("nodeid: host id: %w", err)
	}
	if id := normalizeHostID(raw); id != "" {
		return id, nil
	}
	return fingerprintFallback()
}

func normalizeHostID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return s
}

func fingerprintFallback() (string, error) {
	hn, err := os.Hostname()
	if err != nil {
		hn = "unknown-host"
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("nodeid: fingerprint interfaces: %w", err)
	}
	var macs []string
	for _, ni := range ifaces {
		if ni.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(ni.HardwareAddr) >= 6 {
			macs = append(macs, ni.HardwareAddr.String())
		}
	}
	sort.Strings(macs)
	sum := sha256.Sum256([]byte(strings.Join([]string{hn, strings.Join(macs, ",")}, "|")))
	return "orch-" + hex.EncodeToString(sum[:16]), nil
}
