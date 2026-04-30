package orchvpn

import (
	"strings"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// ClientConfig holds workstation daemon settings.
type ClientConfig struct {
	// ControlPlaneURL is the orch HTTPS base (e.g. https://orch.example:17443).
	ControlPlaneURL string
	// BearerToken is sent as Authorization: Bearer when non-empty (orch-server auth).
	BearerToken string
	// HealthPeriod is how often to re-check control plane reachability when the tunnel is idle.
	HealthPeriodSec int
	// EnableTUN opens a system TUN and forwards encap-v0 IPv4 when the tunnel is up (requires privileges).
	EnableTUN bool
	// TUNName overrides the OS default interface name when non-empty (see tun package / ifname_*.go).
	TUNName string
}

func (c *ClientConfig) normalized() (ClientConfig, error) {
	out := *c
	out.ControlPlaneURL = strings.TrimRight(strings.TrimSpace(c.ControlPlaneURL), "/")
	if out.ControlPlaneURL == "" {
		return ClientConfig{}, oopsx.B("orchvpn").Errorf("control plane URL is required")
	}
	if out.HealthPeriodSec <= 0 {
		out.HealthPeriodSec = 60
	}
	out.TUNName = strings.TrimSpace(out.TUNName)
	return out, nil
}

// ServerConfig holds the orch-vpn serve standalone listener settings.
type ServerConfig struct {
	// ListenUDP is the UDP address for encapsulated tunnel traffic (e.g. ":15888").
	ListenUDP string
}

// ListenUDPOrDefault returns ListenUDP trimmed, or ":15888" when empty.
func (c ServerConfig) ListenUDPOrDefault() string {
	s := strings.TrimSpace(c.ListenUDP)
	if s == "" {
		return ":15888"
	}
	return s
}
