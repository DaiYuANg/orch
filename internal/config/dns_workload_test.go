package config_test

import (
	"slices"
	"testing"

	"github.com/lyonbrown4d/orch/internal/config"
)

func TestDNSWorkloadNameserver(t *testing.T) {
	t.Parallel()

	cfg := config.DNSConfig{Listen: "127.0.0.1:15353"}
	if got, ok := cfg.WorkloadNameserver(); ok || got != "" {
		t.Fatalf("default workload nameserver = %q %v, want disabled", got, ok)
	}

	cfg.Workload.Nameserver = "172.17.0.1:53"
	got, ok := cfg.WorkloadNameserver()
	if !ok || got != "172.17.0.1" {
		t.Fatalf("explicit workload nameserver = %q %v", got, ok)
	}

	cfg.Workload.Nameserver = ""
	cfg.Listen = "10.0.0.10:53"
	got, ok = cfg.WorkloadNameserver()
	if !ok || got != "10.0.0.10" {
		t.Fatalf("inferred workload nameserver = %q %v", got, ok)
	}
}

func TestDNSWorkloadSearchDomainList(t *testing.T) {
	t.Parallel()

	cfg := config.DNSConfig{Zone: "Orch.Local."}
	got := cfg.WorkloadSearchDomainList("Demo").Values()
	want := []string{"demo.svc.orch.local", "svc.orch.local", "orch.local"}
	if !slices.Equal(got, want) {
		t.Fatalf("search = %#v, want %#v", got, want)
	}

	cfg.Workload.Search = []string{"Custom.Local.", "custom.local", "svc.orch.local"}
	got = cfg.WorkloadSearchDomainList("ignored").Values()
	want = []string{"custom.local", "svc.orch.local"}
	if !slices.Equal(got, want) {
		t.Fatalf("custom search = %#v, want %#v", got, want)
	}
}

func TestDNSWorkloadUpstreamList(t *testing.T) {
	t.Parallel()

	cfg := config.DNSConfig{}
	cfg.Workload.Upstream = []string{
		"1.1.1.1",
		"8.8.8.8:5353",
		"[2001:4860:4860::8888]:53",
		"1.1.1.1:53",
		"not-an-ip",
	}

	got := cfg.WorkloadUpstreamList(t.Context()).Values()
	want := []string{
		"1.1.1.1:53",
		"8.8.8.8:5353",
		"[2001:4860:4860::8888]:53",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("upstream = %#v, want %#v", got, want)
	}
}
