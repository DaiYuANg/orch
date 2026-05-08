package dnssvc

import (
	"testing"

	"github.com/daiyuang/orch/internal/config"
)

func TestWorkloadAdvertiseAddress(t *testing.T) {
	t.Parallel()

	svc := &Service{cfg: config.DNSConfig{Enabled: true}}
	if got := svc.WorkloadAdvertiseAddress("127.0.0.1"); got != "127.0.0.1" {
		t.Fatalf("fallback advertise address = %q", got)
	}

	svc.cfg.Workload.AdvertiseAddress = "10.0.0.10"
	if got := svc.WorkloadAdvertiseAddress("127.0.0.1"); got != "10.0.0.10" {
		t.Fatalf("configured advertise address = %q", got)
	}
}
