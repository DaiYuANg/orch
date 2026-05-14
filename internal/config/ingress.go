package config

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
)

type IngressConfig struct {
	Enabled bool           `json:"enabled"`
	Addr    string         `json:"addr,omitempty"`
	Listen  []string       `json:"listen,omitempty"`
	TLS     IngressTLSAuto `json:"tls"` // Let's Encrypt via autocert (TLS-ALPN-01 on HTTPS listeners)
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
