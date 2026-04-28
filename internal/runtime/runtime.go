package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arcgolabs/collectionx/mapping"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

type Provider interface {
	Kind() deployv1.RuntimeKind
	Deploy(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload) error
	Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error
}

type Manager struct {
	logger    *slog.Logger
	providers *mapping.ConcurrentMap[deployv1.RuntimeKind, Provider]
}

func NewManager(logger *slog.Logger, providers ...Provider) *Manager {
	idx := mapping.NewConcurrentMapWithCapacity[deployv1.RuntimeKind, Provider](len(providers))
	for _, p := range providers {
		idx.Set(p.Kind(), p)
	}
	return &Manager{
		logger:    logger,
		providers: idx,
	}
}

func (m *Manager) Deploy(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload) error {
	p, ok := m.providers.Get(workload.Runtime)
	if !ok {
		return fmt.Errorf("runtime provider not registered: %s", workload.Runtime)
	}
	m.logger.Info("deploy workload", "workload", workload.Name, "runtime", workload.Runtime)
	return p.Deploy(ctx, meta, workload)
}

func (m *Manager) Stop(ctx context.Context, runtime deployv1.RuntimeKind, meta deployv1.Metadata, workloadName string) error {
	p, ok := m.providers.Get(runtime)
	if !ok {
		return fmt.Errorf("runtime provider not registered: %s", runtime)
	}
	m.logger.Info("stop workload", "workload", workloadName, "runtime", runtime)
	return p.Stop(ctx, meta, workloadName)
}
