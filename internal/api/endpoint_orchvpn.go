package api

import (
	"context"
	"strings"

	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
)

const orchVPNEncapV0 = "orch-vpn/encap-v0"

// OrchVPNBootstrapEndpoint serves GET /api/v1/orch-vpn/bootstrap for workstation daemons.
type OrchVPNBootstrapEndpoint struct {
	cfg config.Config
	dns *dnssvc.Service
}

func NewOrchVPNBootstrapEndpoint(cfg config.Config, dns *dnssvc.Service) *OrchVPNBootstrapEndpoint {
	return &OrchVPNBootstrapEndpoint{cfg: cfg, dns: dns}
}

func (e *OrchVPNBootstrapEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/orch-vpn/bootstrap",
		Description: "orch-vpn bootstrap metadata (tunnel port, encap version, DNS zone).",
		Tags:        httpx.Tags("orch-vpn"),
	}
}

func (e *OrchVPNBootstrapEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"orch-vpn"}, "orchVPNBootstrap",
		"Fetch orch-vpn tunnel parameters",
		"Returns encap version (orch-vpn/encap-v0), UDP port, MTU, DNS zone, and container_routes (/32 per workload IPv4 known to local DNS). When orch_vpn.enabled, the gateway accepts ORC0 frames (heartbeat + optional IPv4-in-UDP observation)."))
}

func (e *OrchVPNBootstrapEndpoint) handle(_ context.Context, _ *EmptyInput) (*OrchVPNBootstrapOutput, error) {
	v := e.cfg.OrchVPN
	port := v.TunnelUDPPort()
	out := &OrchVPNBootstrapOutput{}
	out.Body.Enabled = v.Enabled
	out.Body.APIVersion = "0"
	out.Body.Encap = orchVPNEncapV0
	out.Body.MTU = 1280
	out.Body.TunnelUDPPort = port
	z := strings.TrimSpace(e.cfg.DNS.Zone)
	z = strings.TrimRight(z, ".")
	if z == "" {
		z = "orch.local"
	}
	out.Body.DNSZone = z
	if e.dns != nil {
		routes := e.dns.WorkloadIPv4HostRouteList()
		if routes.Len() > 0 {
			out.Body.ContainerRoutes = routes
		}
	}
	return out, nil
}
