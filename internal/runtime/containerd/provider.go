package containerd

import (
	"log/slog"
	"path/filepath"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
)

// Provider runs workloads via containerd CRI sandboxes (linux) and registers them in orch DNS.
type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
	root   string
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		logger: logger,
		dns:    dns,
		root:   filepath.Join(config.DefaultDataRoot(), "runtime", "containerd"),
	}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeContainerd
}
