package config

import (
	"log/slog"
	"net"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/samber/lo"
)

// FixLoopbackHost turns bind addresses like ":17443" or "0.0.0.0:80" into a loopback host:port for URLs in logs.
func FixLoopbackHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// LogHTTPServerReachablePaths logs HTTP API and metrics URLs after the control-plane listener is up.
func LogHTTPServerReachablePaths(logger *slog.Logger, cfg Config) {
	if logger == nil {
		return
	}
	httpAddr := FixLoopbackHost(cfg.HTTP.Addr)
	if httpAddr == "" {
		logger.Warn("http reachable paths skipped", "reason", "http.addr_empty")
		return
	}
	apiBase := "http://" + httpAddr
	// OpenAPI doc paths must stay in sync with internal/httpserver (OpenAPIJSONPath, OpenAPIDocsPath).
	attrs := list.NewList[any]()
	attrs.Add(
		slog.String("control_api", apiBase+"/api"),
		slog.String("health", apiBase+"/api/health"),
		slog.String("openapi_json", apiBase+"/openapi.json"),
		slog.String("swagger_ui", apiBase+"/swagger-ui"),
	)

	if cfg.Observability.Prometheus.Enabled {
		attrs.Add(slog.String("metrics", apiBase+NormalizePrometheusPath(cfg.Observability.Prometheus.Path)))
	}

	logArgs := append([]any{slog.String("component", "httpserver")}, attrs.Values()...)
	logger.Info("lifecycle reachable paths", logArgs...)
}

// LogIngressReachablePaths logs ingress URLs after the ingress listener(s) are up.
func LogIngressReachablePaths(logger *slog.Logger, cfg Config) {
	if logger == nil || !cfg.Ingress.Enabled {
		return
	}

	urls := IngressReachabilityURLList(cfg.Ingress)
	if urls.Len() == 0 {
		return
	}
	logger.Info("lifecycle reachable paths", "component", "ingress", "urls", urls.Values())
}

// LogDNSReachablePaths logs where the DNS server listens after it has started.
func LogDNSReachablePaths(logger *slog.Logger, cfg Config) {
	if logger == nil || !cfg.DNS.Enabled {
		return
	}
	listen := strings.TrimSpace(cfg.DNS.Listen)
	if listen == "" {
		return
	}
	logger.Info("lifecycle reachable paths", "component", "dns", "listen", listen, "zone", dnsZoneFromConfig(cfg.DNS))
}

func dnsZoneFromConfig(d DNSConfig) string {
	return lo.CoalesceOrEmpty(strings.TrimSpace(d.Zone), "orch.local")
}

// LogRaftReachablePaths logs Raft transport bind after the node has started.
func LogRaftReachablePaths(logger *slog.Logger, cfg Config) {
	if logger == nil {
		return
	}
	bind := strings.TrimSpace(cfg.Raft.Bind)
	if bind == "" {
		return
	}
	advertise := strings.TrimSpace(cfg.Raft.Advertise)
	logger.Info("lifecycle reachable paths", "component", "raft", "transport_bind", bind, "transport_advertise", advertise)
}

// LogSchedulerReachableContext logs scheduler cadence (no external URL).
func LogSchedulerReachableContext(logger *slog.Logger, cfg Config) {
	if logger == nil {
		return
	}
	logger.Info("lifecycle reachable paths", "component", "scheduler",
		"heartbeat_interval", cfg.Scheduler.HeartbeatInterval,
		"note", "in-process gocron; leader-only mode controlled by scheduler config")
}

func ingressReachabilityURLList(cfg Config) *list.List[string] {
	if !cfg.Ingress.Enabled {
		return list.NewList[string]()
	}
	return IngressReachabilityURLList(cfg.Ingress)
}
func prometheusMetricsAttr(apiBase string, cfg Config) (slog.Attr, bool) {
	if !cfg.Observability.Prometheus.Enabled {
		return slog.Attr{}, false
	}
	return slog.String("metrics", apiBase+NormalizePrometheusPath(cfg.Observability.Prometheus.Path)), true
}

func appendReachabilityDNS(attrs *list.List[any], cfg Config) {
	if cfg.DNS.Enabled && strings.TrimSpace(cfg.DNS.Listen) != "" {
		attrs.Add(slog.String("dns_listen", cfg.DNS.Listen))
		attrs.Add(slog.String("dns_zone", dnsZoneFromConfig(cfg.DNS)))
	}
}

func appendReachabilityRaft(attrs *list.List[any], cfg Config) {
	if strings.TrimSpace(cfg.Raft.Bind) != "" {
		attrs.Add(slog.String("raft_transport", cfg.Raft.Bind))
		if advertise := strings.TrimSpace(cfg.Raft.Advertise); advertise != "" {
			attrs.Add(slog.String("raft_advertise", advertise))
		}
	}
}

func appendReachabilityScheduler(attrs *list.List[any], cfg Config) {
	attrs.Add(
		slog.String("scheduler_heartbeat_interval", cfg.Scheduler.HeartbeatInterval),
		slog.String("scheduler_note", "in-process gocron; leader-only mode controlled by scheduler config"),
	)
}

// LogReachableEndpoints writes one structured log line with all URLs (optional summary after startup).
func LogReachableEndpoints(logger *slog.Logger, cfg Config) {
	if logger == nil {
		return
	}

	httpAddr := FixLoopbackHost(cfg.HTTP.Addr)
	if httpAddr == "" {
		logger.Warn("reachable endpoints skipped", "reason", "http.addr_empty")
		return
	}
	apiBase := "http://" + httpAddr
	// OpenAPI doc paths must stay in sync with internal/httpserver (OpenAPIJSONPath, OpenAPIDocsPath).
	attrs := list.NewList[any]()
	attrs.Add(
		slog.String("control_api", apiBase+"/api"),
		slog.String("health", apiBase+"/api/health"),
		slog.String("openapi_json", apiBase+"/openapi.json"),
		slog.String("swagger_ui", apiBase+"/swagger-ui"),
	)

	if attr, ok := prometheusMetricsAttr(apiBase, cfg); ok {
		attrs.Add(attr)
	}

	if ingressURLs := ingressReachabilityURLList(cfg); ingressURLs.Len() > 0 {
		attrs.Add(slog.Any("ingress", ingressURLs.Values()))
	}

	appendReachabilityDNS(attrs, cfg)
	appendReachabilityRaft(attrs, cfg)
	appendReachabilityScheduler(attrs, cfg)

	logger.Info("lifecycle reachability summary", attrs.Values()...)
}
