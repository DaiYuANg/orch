package config

import (
	"slices"
	"testing"
)

func TestIngressPlainAndTLSListenAddrs(t *testing.T) {
	t.Parallel()

	c := IngressConfig{
		Enabled: true,
		Listen:  []string{":80", ":443"},
		TLS: IngressTLSAuto{
			Enabled: true,
			Domains: []string{"example.com"},
		},
	}
	if got := c.TLSListenAddrs(); len(got) != 1 || got[0] != ":443" {
		t.Fatalf("TLSListenAddrs: %#v", got)
	}
	wantPlain := []string{":80"}
	if got := c.PlainListenAddrs(); !slices.Equal(got, wantPlain) {
		t.Fatalf("PlainListenAddrs: %#v want %#v", got, wantPlain)
	}

	c2 := IngressConfig{
		Enabled: true,
		Listen:  []string{":8080"},
		TLS: IngressTLSAuto{
			Enabled: true,
			Listen:  []string{":8443"},
			Domains: []string{"a.example"},
		},
	}
	if got := c2.TLSListenAddrs(); len(got) != 1 || got[0] != ":8443" {
		t.Fatalf("TLSListenAddrs custom: %#v", got)
	}
	if got := c2.PlainListenAddrs(); len(got) != 1 || got[0] != ":8080" {
		t.Fatalf("PlainListenAddrs: %#v", got)
	}
}

func TestIngressReachabilityURLsWithTLS(t *testing.T) {
	t.Parallel()
	u := IngressReachabilityURLs(IngressConfig{
		Enabled: true,
		Listen:  []string{":80"},
		TLS: IngressTLSAuto{
			Enabled: true,
			Domains: []string{"orch.example"},
		},
	})
	if len(u) < 2 {
		t.Fatalf("urls: %#v", u)
	}
}
