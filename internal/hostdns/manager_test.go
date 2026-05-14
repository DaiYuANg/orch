package hostdns_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/hostdns"
)

func TestConfigFromOrch(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.DNS.Listen = "0.0.0.0:53"
	cfg.DNS.Zone = "Orch.Local."

	got, err := hostdns.ConfigFromOrch(cfg)
	if err != nil {
		t.Fatalf("ConfigFromOrch: %v", err)
	}
	if got.Zone != "orch.local" || got.Nameserver != "127.0.0.1" || got.Port != 53 {
		t.Fatalf("config = %#v", got)
	}
}

func TestConfigFromOrchRejectsNonIPListenHost(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.DNS.Listen = "localhost:53"
	if _, err := hostdns.ConfigFromOrch(cfg); err == nil {
		t.Fatal("expected non-IP listen host error")
	}
}

func TestEmbeddedHostDNSTemplates(t *testing.T) {
	t.Parallel()

	data := hostdns.TemplateData{
		Zone:       "orch.local",
		Namespace:  ".orch.local",
		Nameserver: "127.0.0.1",
		DNSServer:  "127.0.0.1:15353",
		Port:       53,
	}
	for _, name := range []string{
		"linux-resolved.conf.tmpl",
		"darwin-resolver.tmpl",
		"windows-install.ps1",
		"windows-uninstall.ps1",
		"windows-status.ps1",
	} {
		if got, err := hostdns.RenderTemplate(name, data); err != nil {
			t.Fatalf("%s render: %v", name, err)
		} else if got == "" {
			t.Fatalf("%s render empty", name)
		}
	}
}

func TestDNSServerEndpointOmitsDefaultPort(t *testing.T) {
	t.Parallel()

	got := hostdns.DNSServerEndpoint(hostdns.Config{Nameserver: "127.0.0.1", Port: 53})
	if got != "127.0.0.1" {
		t.Fatalf("endpoint = %q, want bare nameserver", got)
	}
}

func TestDNSServerEndpointIncludesCustomPort(t *testing.T) {
	t.Parallel()

	got := hostdns.DNSServerEndpoint(hostdns.Config{Nameserver: "127.0.0.1", Port: 15353})
	if got != "127.0.0.1:15353" {
		t.Fatalf("endpoint = %q, want nameserver:port", got)
	}
}
