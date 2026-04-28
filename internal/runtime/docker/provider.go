package docker

import (
	"context"
	"log/slog"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

type Provider struct {
	logger *slog.Logger
}

func NewProvider(logger *slog.Logger) *Provider {
	return &Provider{logger: logger}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeDocker
}

func (p *Provider) Deploy(_ context.Context, workload deployv1.Workload) error {
	p.logger.Info("docker deploy simulated", "workload", workload.Name, "image", workload.Run.Image)
	return nil
}

func (p *Provider) Stop(_ context.Context, workloadName string) error {
	p.logger.Info("docker stop simulated", "workload", workloadName)
	return nil
}

