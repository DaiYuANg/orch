package config

import (
	"net"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
)

// IngressReachabilityURLs builds http(s) URLs for logging: plain listeners, TLS bind addresses, and https://<domain>/ for each ingress.tls.domains entry when autocert is enabled.
func IngressReachabilityURLs(ing IngressConfig) []string {
	return IngressReachabilityURLList(ing).Values()
}

// IngressReachabilityURLList builds http(s) URLs for logging as a collectionx list.
func IngressReachabilityURLList(ing IngressConfig) *list.List[string] {
	if !ing.Enabled {
		return list.NewList[string]()
	}
	urls := list.NewList[string]()
	urls.Merge(IngressURLListFromAddrList(ing.PlainListenAddrList()))
	if ing.TLS.Enabled {
		ing.TLSAutocertDomainList().Range(func(_ int, d string) bool {
			urls.Add("https://" + strings.TrimSuffix(d, "/") + "/")
			return true
		})
		urls.Merge(IngressURLListFromAddrList(ing.TLSListenAddrList()))
	}
	return uniqueStringList(urls)
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
	return IngressURLListFromAddrList(list.NewList(addrs...)).Values()
}

// IngressURLListFromAddrList builds URLs for ingress reachability logging from a collectionx address list.
func IngressURLListFromAddrList(addrs *list.List[string]) *list.List[string] {
	if addrs.Len() == 0 {
		return list.NewList[string]()
	}
	urls := list.FilterMapList(addrs, func(_ int, a string) (string, bool) {
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
	})
	return uniqueStringList(urls)
}

func uniqueStringList(values *list.List[string]) *list.List[string] {
	seen := set.NewOrderedSetWithCapacity[string](values.Len())
	values.Range(func(_ int, value string) bool {
		seen.Add(value)
		return true
	})
	out := list.NewListWithCapacity[string](seen.Len())
	seen.Range(func(value string) bool {
		out.Add(value)
		return true
	})
	return out
}
