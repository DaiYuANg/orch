package orchvpn

import "testing"

func TestClientConfig_normalized(t *testing.T) {
	t.Parallel()
	_, err := (&ClientConfig{}).normalized()
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	got, err := (&ClientConfig{ControlPlaneURL: "http://a/"}).normalized()
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
