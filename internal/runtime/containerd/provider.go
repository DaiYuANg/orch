package containerd

import (
	"log/slog"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
)

// Provider runs workloads via containerd (linux) and registers them in orch DNS.
type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{logger: logger, dns: dns}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeContainerd
}
