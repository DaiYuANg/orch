package orchvpn_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/orchvpn"
)

func TestClientConfig_normalized(t *testing.T) {
	t.Parallel()
	_, err := (&orchvpn.ClientConfig{}).Normalize()
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	got, err := (&orchvpn.ClientConfig{ControlPlaneURL: "http://a/"}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if got.ControlPlaneURL != "http://a" {
		t.Fatalf("trim slash: %q", got.ControlPlaneURL)
	}
	if got.HealthPeriodSec != 60 {
		t.Fatalf("default period: %d", got.HealthPeriodSec)
	}
}
