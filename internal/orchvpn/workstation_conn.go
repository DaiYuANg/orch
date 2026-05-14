package orchvpn

import (
	"os"
	"strings"

	"github.com/lyonbrown4d/orch/internal/apiclient"
)

// WorkstationConn is the Cobra/CLI boundary for the orch-vpn workstation process (injectable into the dix graph).
type WorkstationConn struct {
	ServerURL       string
	Token           string
	HealthPeriodSec int
	EnableTUN       bool
	TUNName         string
}

// ClientConfig returns normalized dial settings for the control plane and daemon loop.
func (c WorkstationConn) ClientConfig() (ClientConfig, error) {
	url := strings.TrimSpace(c.ServerURL)
	if url == "" {
		url = apiclient.DefaultBaseURL()
	}
	tok := strings.TrimSpace(c.Token)
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("ORCH_TOKEN"))
	}
	cc := ClientConfig{
		ControlPlaneURL: url,
		BearerToken:     tok,
		HealthPeriodSec: c.HealthPeriodSec,
		EnableTUN:       c.EnableTUN,
		TUNName:         c.TUNName,
	}
	return cc.normalized()
}
