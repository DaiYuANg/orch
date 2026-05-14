package config_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/config"
)

func TestOrchVPNConfig_TunnelUDPPort(t *testing.T) {
	t.Parallel()
	if got := (config.OrchVPNConfig{TunnelListenUDP: ":9099"}).TunnelUDPPort(); got != 9099 {
		t.Fatalf("got %d", got)
	}
	if got := (config.OrchVPNConfig{}).TunnelUDPPort(); got != 15888 {
		t.Fatalf("default got %d", got)
	}
}
