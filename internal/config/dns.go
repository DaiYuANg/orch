package config

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
)

type OrchVPNConfig struct {
	Enabled         bool   `json:"enabled"`
	TunnelListenUDP string `json:"tunnel_listen_udp,omitempty"` // e.g. ":15888" or "0.0.0.0:15888"
}

// ListenUDPOrDefault returns tunnel_listen_udp or ":15888".
func (c OrchVPNConfig) ListenUDPOrDefault() string {
	s := strings.TrimSpace(c.TunnelListenUDP)
	if s == "" {
		return ":15888"
	}
	return s
}

// TunnelUDPPort returns the UDP port from ListenUDPOrDefault (default 15888 if parse fails).
func (c OrchVPNConfig) TunnelUDPPort() int {
	addr := c.ListenUDPOrDefault()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 15888
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 {
		return 15888
	}
	return p
}

// DNSConfig matches koanf paths like dns.data.path (env ORCH_DNS_DATA_PATH → dns.data.path).
type DNSConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Data    struct {
		Path string `json:"path"`
	} `json:"data"`
	Zone     string            `json:"zone,omitempty"`
	Workload DNSWorkloadConfig `json:"workload"`
}

// DNSWorkloadConfig controls how managed workloads discover orch DNS.
// nameserver must be reachable by the workload on the standard DNS port (53);
// resolv.conf-style resolvers do not support custom ports.
type DNSWorkloadConfig struct {
	Nameserver       string   `json:"nameserver,omitempty"`        // IP injected into container DNS config.
	Search           []string `json:"search,omitempty"`            // Optional search domains; defaults to namespace/service/zone.
	Upstream         []string `json:"upstream,omitempty"`          // Optional DNS upstreams for non-orch names queried by workloads.
	AdvertiseAddress string   `json:"advertise_address,omitempty"` // Address used for host-style runtimes' A records.
}

func (c DNSConfig) ZoneName() string {
	z := strings.Trim(strings.ToLower(strings.TrimSpace(c.Zone)), ".")
	if z == "" {
		return "orch.local"
	}
	return z
}

// WorkloadNameserver returns the nameserver address to inject into container runtimes.
// The returned value never includes a port because container resolvers query port 53.
func (c DNSConfig) WorkloadNameserver() (string, bool) {
	if ns, ok := normalizeDNSNameserver(c.Workload.Nameserver, true); ok {
		return ns, true
	}
	return workloadNameserverFromListen(c.Listen)
}

func (c DNSConfig) WorkloadSearchDomainList(namespace string) *list.List[string] {
	if len(c.Workload.Search) > 0 {
		return normalizeDNSDomains(list.NewList(c.Workload.Search...))
	}
	ns := strings.Trim(strings.ToLower(strings.TrimSpace(namespace)), ".")
	if ns == "" {
		ns = "default"
	}
	zone := c.ZoneName()
	return normalizeDNSDomains(list.NewList(
		ns+".svc."+zone,
		"svc."+zone,
		zone,
	))
}

func (c DNSConfig) WorkloadAdvertiseAddress() string {
	return strings.TrimSpace(c.Workload.AdvertiseAddress)
}

func (c DNSConfig) WorkloadUpstreamList(ctx context.Context) *list.List[string] {
	return normalizeDNSUpstreams(ctx, list.NewList(c.Workload.Upstream...))
}

func workloadNameserverFromListen(listen string) (string, bool) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || port != "53" {
		return "", false
	}
	return normalizeDNSNameserver(host, false)
}

func normalizeDNSNameserver(raw string, allowLoopback bool) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		if port != "" && port != "53" {
			return "", false
		}
		s = host
	}
	s = strings.Trim(strings.TrimSpace(s), "[]")
	ip := net.ParseIP(s)
	if ip == nil {
		return "", false
	}
	if !allowLoopback && (ip.IsLoopback() || ip.IsUnspecified()) {
		return "", false
	}
	return ip.String(), true
}

func normalizeDNSUpstreams(ctx context.Context, upstreams *list.List[string]) *list.List[string] {
	seen := set.NewSet[string]()
	return list.FilterMapList(upstreams, func(_ int, upstream string) (string, bool) {
		u, ok := normalizeDNSUpstream(ctx, upstream)
		if !ok || seen.Contains(u) {
			return "", false
		}
		seen.Add(u)
		return u, true
	})
}

func normalizeDNSUpstream(ctx context.Context, raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	host, port, err := net.SplitHostPort(s)
	if err != nil {
		host = s
		port = "53"
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || net.ParseIP(host) == nil {
		return "", false
	}
	if port == "" {
		port = "53"
	}
	p, err := net.DefaultResolver.LookupPort(ctx, "udp", port)
	if err != nil || p <= 0 || p > 65535 {
		return "", false
	}
	return net.JoinHostPort(net.ParseIP(host).String(), strconv.Itoa(p)), true
}

func normalizeDNSDomains(domains *list.List[string]) *list.List[string] {
	seen := set.NewSet[string]()
	return list.FilterMapList(domains, func(_ int, domain string) (string, bool) {
		d := strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
		if d == "" || seen.Contains(d) {
			return "", false
		}
		seen.Add(d)
		return d, true
	})
}
