package config

import (
	"log/slog"
	"net"
	"strings"

	"github.com/arcgolabs/collectionx/list"
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

// LogReachableEndpoints writes one structured log line with URLs operators typically open from localhost.
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

	attrs := list.NewList[any]()
	attrs.Add(
		slog.String("control_api", apiBase+"/api"),
		slog.String("health", apiBase+"/api/health"),
		slog.String("openapi_json", apiBase+"/openapi.json"),
		slog.String("swagger_ui", apiBase+"/swagger-ui"),
	)

	if cfg.Observability.PrometheusEnabled {
		path := strings.TrimSpace(cfg.Observability.PrometheusPath)
		if path == "" {
			path = "/metrics"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		attrs.Add(slog.String("metrics", apiBase+path))
	}

	if cfg.Ingress.Enabled {
		addrs := cfg.Ingress.ListenAddrs()
		urls := list.NewListWithCapacity[string](len(addrs))
		for _, a := range addrs {
			d := FixLoopbackHost(a)
			if d == "" {
				continue
			}
			_, port, err := net.SplitHostPort(d)
			if err != nil {
				urls.Add("http://" + d + "/")
				continue
			}
			scheme := "http"
			if port == "443" {
				scheme = "https"
			}
			urls.Add(scheme + "://" + d + "/")
		}
		if urls.Len() > 0 {
			attrs.Add(slog.Any("ingress", urls.Values()))
		}
	}

	if cfg.DNS.Enabled && strings.TrimSpace(cfg.DNS.Listen) != "" {
		attrs.Add(slog.String("dns_listen", cfg.DNS.Listen))
	}

	if cfg.Raft.Enabled && strings.TrimSpace(cfg.Raft.Bind) != "" {
		attrs.Add(slog.String("raft_transport", cfg.Raft.Bind))
	}

	logger.Info("reachable endpoints", attrs.Values()...)
}
