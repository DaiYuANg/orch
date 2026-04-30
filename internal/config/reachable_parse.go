package config

import (
	"net"
	"strings"

	"github.com/samber/lo"
)

// IngressReachabilityURLs builds http(s) URLs for logging: plain listeners, TLS bind addresses, and https://<domain>/ for each ingress.tls.domains entry when autocert is enabled.
func IngressReachabilityURLs(ing IngressConfig) []string {
	if !ing.Enabled {
		return nil
	}
	var urls []string
	urls = append(urls, IngressURLsFromAddrs(ing.PlainListenAddrs())...)
	if ing.TLS.Enabled {
		for _, d := range ing.TLSAutocertDomains() {
			urls = append(urls, "https://"+strings.TrimSuffix(d, "/")+"/")
		}
		urls = append(urls, IngressURLsFromAddrs(ing.TLSListenAddrs())...)
	}
	return lo.Uniq(urls)
}

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
