package config

import (
	"net"
	"strings"

	"github.com/samber/lo"
)

// NormalizePrometheusPath returns a stable path attribute for prometheus.path.
// Trims and ensures a leading slash; empty or "/" becomes "/metrics".
func NormalizePrometheusPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p == "/" {
		return "/metrics"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// IngressURLsFromAddrs builds URLs for ingress reachability logging (same rules as ingressReachabilityURLs).
func IngressURLsFromAddrs(addrs []string) []string {
	if len(addrs) == 0 {
		return nil
	}
	return lo.Uniq(lo.FilterMap(addrs, func(a string, _ int) (string, bool) {
		d := FixLoopbackHost(strings.TrimSpace(a))
		if d == "" {
			return "", false
		}
		_, port, err := net.SplitHostPort(d)
		if err != nil {
			return "http://" + d + "/", true
		}
		scheme := "http"
		if port == "443" {
			scheme = "https"
		}
		return scheme + "://" + d + "/", true
	}))
}
