package config

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/arcgolabs/configx"
)

type Config struct {
	App           AppConfig           `json:"app"`
	Env           string              `json:"env"`
	Log           LogConfig           `json:"log"`
	HTTP          HTTPConfig          `json:"http"`
	Observability ObservabilityConfig `json:"observability"`
	Ingress       IngressConfig       `json:"ingress"`
	DNS           DNSConfig           `json:"dns"`
	OrchVPN       OrchVPNConfig       `json:"orch_vpn"`
	Scheduler     SchedulerConfig     `json:"scheduler"`
	Cluster       ClusterConfig       `json:"cluster"`
	Auth          AuthConfig          `json:"auth"`
	Raft          RaftConfig          `json:"raft"`
}

type AppConfig struct {
	Name string `json:"name"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type HTTPConfig struct {
	Addr string `json:"addr"`
}

// ObservabilityConfig matches koanf paths like observability.prometheus.enabled (env ORCH_OBSERVABILITY_PROMETHEUS_ENABLED).
type ObservabilityConfig struct {
	Prometheus struct {
		Enabled         bool   `json:"enabled"`
		Path            string `json:"path"`
		NativeHistogram bool   `json:"native_histogram"` // classic + Prometheus native histogram on request duration (scrapers need native support)
	} `json:"prometheus"`
	// OTLP exports traces and metrics via OpenTelemetry (grpc default :4317, http default http://localhost:4318).
	OTLP struct {
		Enabled     bool   `json:"enabled"`
		Protocol    string `json:"protocol"` // grpc or http
		Endpoint    string `json:"endpoint"` // host:port for grpc; URL or host:port for http
		Insecure    bool   `json:"insecure"` // plaintext grpc; for http, use http:// or set insecure with host:port
		ServiceName string `json:"service_name"`
	} `json:"otlp"`
}

type IngressConfig struct {
	Enabled bool           `json:"enabled"`
	Addr    string         `json:"addr,omitempty"`
	Listen  []string       `json:"listen,omitempty"`
	TLS     IngressTLSAuto `json:"tls,omitempty"` // Let's Encrypt via autocert (TLS-ALPN-01 on HTTPS listeners)
}

// IngressTLSAuto configures automatic TLS certificates (Let's Encrypt) using golang.org/x/crypto/acme/autocert.
// When enabled, ingress.tls.listen (default [":443"]) terminates TLS; those addresses are excluded from plain
// ingress.listen via PlainListenAddrs() so :80 can stay HTTP and :443 HTTPS without double-binding.
//
// Issuance uses TLS-ALPN-01 on the HTTPS listeners (ACME HTTP-01 on :80 is not required for LE v2 with this setup).
type IngressTLSAuto struct {
	Enabled  bool     `json:"enabled"`
	Listen   []string `json:"listen,omitempty"`    // TLS bind addresses; default [":443"] when enabled and empty
	Domains  []string `json:"domains,omitempty"`   // host names for certificates (autocert host whitelist)
	Email    string   `json:"email,omitempty"`     // ACME registration email (recommended for production)
	CacheDir string   `json:"cache_dir,omitempty"` // PEM cache; default <data-dir>/autocert
	Staging  bool     `json:"staging,omitempty"`   // Let's Encrypt staging directory (for testing)
}

// IngressRoute is a compiled ingress row (from deploy documents' ingresses block) used by the data plane.
// The first route with a matching path_prefix wins. StripPrefix defaults to PathPrefix when empty.
//
// Specify either upstream (single) or upstreams (one or more). Multiple upstreams use arcgolabs/vale
// round-robin (see lb).
type IngressRoute struct {
	PathPrefix  string   `json:"path_prefix"`
	Upstream    string   `json:"upstream,omitempty"`
	Upstreams   []string `json:"upstreams,omitempty"`
	StripPrefix string   `json:"strip_prefix,omitempty"`
	LB          string   `json:"lb,omitempty"` // round_robin (default); other values rejected for now
}

// UpstreamEndpoints returns non-empty upstream URLs: Upstreams if set, else a single Upstream.
func (r *IngressRoute) UpstreamEndpoints() *list.List[string] {
	out := list.FilterMapList(list.NewList(r.Upstreams...), func(_ int, u string) (string, bool) {
		u = strings.TrimSpace(u)
		if u == "" {
			return "", false
		}
		return u, true
	})
	if out.Len() > 0 {
		return out
	}
	if u := strings.TrimSpace(r.Upstream); u != "" {
		return list.NewList(u)
	}
	return list.NewList[string]()
}

// LBPolicy returns the load-balancing policy name (lowercase). Empty defaults to round_robin.
func (r *IngressRoute) LBPolicy() string {
	p := strings.TrimSpace(r.LB)
	if p == "" {
		return "round_robin"
	}
	return strings.ToLower(p)
}

// ListenAddrs returns configured plain bind addresses: explicit Listen, else single Addr, else defaults ":80" and ":443".
// When TLS is enabled, use PlainListenAddrs() for HTTP-only binds and TLSListenAddrs() for HTTPS.
func (c IngressConfig) ListenAddrs() []string {
	return c.ListenAddrList().Values()
}

// ListenAddrList returns configured plain bind addresses as a collectionx list.
func (c IngressConfig) ListenAddrList() *list.List[string] {
	if len(c.Listen) > 0 {
		return list.NewList(c.Listen...)
	}
	if strings.TrimSpace(c.Addr) != "" {
		return list.NewList(c.Addr)
	}
	return list.NewList(":80", ":443")
}

// TLSListenAddrs returns TLS bind addresses when ingress.tls.enabled; default [":443"] if tls.listen is empty.
func (c IngressConfig) TLSListenAddrs() []string {
	return c.TLSListenAddrList().Values()
}

// TLSListenAddrList returns TLS bind addresses when ingress.tls.enabled as a collectionx list.
func (c IngressConfig) TLSListenAddrList() *list.List[string] {
	if !c.TLS.Enabled {
		return list.NewList[string]()
	}
	if len(c.TLS.Listen) > 0 {
		out := list.FilterMapList(list.NewList(c.TLS.Listen...), func(_ int, a string) (string, bool) {
			a = strings.TrimSpace(a)
			if a == "" {
				return "", false
			}
			return a, true
		})
		if out.Len() > 0 {
			return out
		}
	}
	return list.NewList(":443")
}

// PlainListenAddrs returns addresses for plaintext HTTP. When autocert is enabled, addresses also listed in
// TLSListenAddrs() (exact string match after trim) are skipped.
func (c IngressConfig) PlainListenAddrs() []string {
	return c.PlainListenAddrList().Values()
}

// PlainListenAddrList returns addresses for plaintext HTTP as a collectionx list.
func (c IngressConfig) PlainListenAddrList() *list.List[string] {
	plain := c.ListenAddrList()
	if !c.TLS.Enabled {
		return plain
	}
	skip := set.NewSet[string]()
	c.TLSListenAddrList().Range(func(_ int, a string) bool {
		if a = strings.TrimSpace(a); a != "" {
			skip.Add(a)
		}
		return true
	})
	return list.FilterMapList(plain, func(_ int, a string) (string, bool) {
		a = strings.TrimSpace(a)
		if a == "" {
			return "", false
		}
		if skip.Contains(a) {
			return "", false
		}
		return a, true
	})
}

// TLSAutocertDomains returns non-empty trimmed host names from ingress.tls.domains.
func (c IngressConfig) TLSAutocertDomains() []string {
	return c.TLSAutocertDomainList().Values()
}

// TLSAutocertDomainList returns non-empty trimmed host names from ingress.tls.domains as a collectionx list.
func (c IngressConfig) TLSAutocertDomainList() *list.List[string] {
	return list.FilterMapList(list.NewList(c.TLS.Domains...), func(_ int, d string) (string, bool) {
		d = strings.TrimSpace(d)
		if d == "" {
			return "", false
		}
		return d, true
	})
}

// OrchVPNConfig enables the orch-vpn tunnel listener and bootstrap metadata (see GET /api/v1/orch-vpn/bootstrap).
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
	Workload DNSWorkloadConfig `json:"workload,omitempty"`
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

func (c DNSConfig) WorkloadUpstreamList() *list.List[string] {
	return normalizeDNSUpstreams(list.NewList(c.Workload.Upstream...))
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

func normalizeDNSUpstreams(upstreams *list.List[string]) *list.List[string] {
	seen := set.NewSet[string]()
	out := list.NewListWithCapacity[string](upstreams.Len())
	upstreams.Range(func(_ int, upstream string) bool {
		u, ok := normalizeDNSUpstream(upstream)
		if !ok || seen.Contains(u) {
			return true
		}
		seen.Add(u)
		out.Add(u)
		return true
	})
	return out
}

func normalizeDNSUpstream(raw string) (string, bool) {
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
	p, err := net.LookupPort("udp", port)
	if err != nil || p <= 0 || p > 65535 {
		return "", false
	}
	return net.JoinHostPort(net.ParseIP(host).String(), strconv.Itoa(p)), true
}

func normalizeDNSDomains(domains *list.List[string]) *list.List[string] {
	seen := set.NewSet[string]()
	out := list.NewListWithCapacity[string](domains.Len())
	domains.Range(func(_ int, domain string) bool {
		d := strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
		if d == "" || seen.Contains(d) {
			return true
		}
		seen.Add(d)
		out.Add(d)
		return true
	})
	return out
}

type SchedulerConfig struct {
	HeartbeatInterval       string `json:"heartbeat_interval,omitempty"`
	ResourceRefreshInterval string `json:"resource_refresh_interval,omitempty"` // cadence for leader to apply local host metrics into Raft
	RaftLeaderOnly          bool   `json:"raft_leader_only,omitempty"`
	MaxConcurrentJobs       uint   `json:"max_concurrent_jobs,omitempty"`
	ConcurrentJobsMode      string `json:"concurrent_jobs_mode,omitempty"`
}

type ClusterConfig struct {
	// Nodes maps node ids to worker API base URLs, for example {"node-a": "http://10.0.0.11:17443"}.
	Nodes map[string]string `json:"nodes,omitempty"`
	// WorkerToken is sent as a bearer token for worker dispatch when remote nodes require HTTP auth.
	WorkerToken string `json:"worker_token,omitempty"`
}

func (c ClusterConfig) NodeURL(nodeID string) (string, bool) {
	id := strings.TrimSpace(nodeID)
	if id == "" || len(c.Nodes) == 0 {
		return "", false
	}
	for k, v := range c.Nodes {
		if strings.TrimSpace(k) != id {
			continue
		}
		url := strings.TrimRight(strings.TrimSpace(v), "/")
		if url == "" {
			return "", false
		}
		return url, true
	}
	return "", false
}

// AuthConfig matches paths auth.enabled and auth.jwt.secret.
type AuthConfig struct {
	Enabled bool `json:"enabled"`
	JWT     struct {
		Secret string `json:"secret"`
	} `json:"jwt"`
}

// RaftConfig matches raft.node.id, raft.badger.dir, raft.bolt.path, raft.snapshot.dir, etc.
type RaftConfig struct {
	Enabled bool `json:"enabled"`
	Node    struct {
		// ID forces this Raft/server identity (omit, "", or "auto" → resolved via nodeid hardware resolver).
		ID string `json:"id"`
	} `json:"node"`
	Bind      string            `json:"bind"`
	Advertise string            `json:"advertise,omitempty"`
	Peers     map[string]string `json:"peers,omitempty"`
	Bootstrap bool              `json:"bootstrap,omitempty"`
	Badger    struct {
		Dir string `json:"dir"`
	} `json:"badger"`
	Bolt struct {
		Path string `json:"path"`
	} `json:"bolt"`
	Snapshot struct {
		Dir string `json:"dir"`
	} `json:"snapshot"`
}

// Load merges defaults, dotenv, optional files, env (ORCH_), then CLI flags when passed via [LoadFromCobra].
func Load(opts ...configx.Option) (Config, error) {
	base := []configx.Option{
		configx.WithTypedDefaults(Default()),
		configx.WithEnvPrefix("ORCH"),
		configx.WithPriority(
			configx.SourceDotenv,
			configx.SourceFile,
			configx.SourceEnv,
			configx.SourceArgs,
		),
		configx.WithValidateLevel(configx.ValidateLevelNone),
	}
	return configx.LoadTErr[Config](append(base, opts...)...)
}

func Default() Config {
	root := DefaultDataRoot()

	var dns DNSConfig
	dns.Enabled = true
	dns.Listen = "127.0.0.1:15353"
	dns.Data.Path = filepath.Join(root, "dnsx.db")
	dns.Zone = "orch.local"

	var obs ObservabilityConfig
	obs.Prometheus.Enabled = true
	obs.Prometheus.Path = "/metrics"
	obs.Prometheus.NativeHistogram = true

	var auth AuthConfig
	auth.Enabled = false
	auth.JWT.Secret = "dev-secret-change-me"

	var raft RaftConfig
	raft.Enabled = true
	// raft.Node.ID empty or "auto" → resolved from hardware at runtime ([nodeid.Resolve]).
	raft.Node.ID = ""
	raft.Bind = "127.0.0.1:7444"
	raft.Advertise = ""
	raft.Peers = map[string]string{}
	raft.Bootstrap = true
	raft.Badger.Dir = filepath.Join(root, "raft-sched")
	raft.Bolt.Path = filepath.Join(root, "raft-meta.db")
	raft.Snapshot.Dir = filepath.Join(root, "raft-snapshots")

	return Config{
		App: AppConfig{
			Name: "orch",
		},
		Env: "dev",
		Log: LogConfig{
			Level: "debug",
		},
		HTTP: HTTPConfig{
			Addr: ":17443",
		},
		Observability: obs,
		Ingress: IngressConfig{
			Enabled: true,
			Listen:  []string{":80", ":443"},
		},
		DNS: dns,
		OrchVPN: OrchVPNConfig{
			Enabled:         false,
			TunnelListenUDP: ":15888",
		},
		Scheduler: SchedulerConfig{
			HeartbeatInterval:       "2m",
			ResourceRefreshInterval: "30s",
			RaftLeaderOnly:          false,
			MaxConcurrentJobs:       0,
			ConcurrentJobsMode:      "reschedule",
		},
		Cluster: ClusterConfig{
			Nodes: map[string]string{},
		},
		Auth: auth,
		Raft: raft,
	}
}
