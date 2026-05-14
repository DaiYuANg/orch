package dnssvc_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dnssvc"
)

func TestWorkloadAdvertiseAddress(t *testing.T) {
	t.Parallel()

	svc := dnssvc.New(config.Config{DNS: config.DNSConfig{Enabled: true}}, nil)
	if got := svc.WorkloadAdvertiseAddress("127.0.0.1"); got != "127.0.0.1" {
		t.Fatalf("fallback advertise address = %q", got)
	}

	cfg := config.DNSConfig{Enabled: true}
	cfg.Workload.AdvertiseAddress = "10.0.0.10"
	svc = dnssvc.New(config.Config{DNS: cfg}, nil)
	if got := svc.WorkloadAdvertiseAddress("127.0.0.1"); got != "10.0.0.10" {
		t.Fatalf("configured advertise address = %q", got)
	}
}
