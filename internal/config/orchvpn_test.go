package config

import "testing"

func TestOrchVPNConfig_TunnelUDPPort(t *testing.T) {
	t.Parallel()
	if got := (OrchVPNConfig{TunnelListenUDP: ":9099"}).TunnelUDPPort(); got != 9099 {
		t.Fatalf("got %d", got)
	}
	if got := (OrchVPNConfig{}).TunnelUDPPort(); got != 15888 {
		t.Fatalf("default got %d", got)
	}
}
