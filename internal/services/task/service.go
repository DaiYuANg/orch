package task

import (
	"context"
	"log/slog"
	"sync"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/services/registry"
)

type Service struct {
	logger     *slog.Logger
	cfg        config.Config
	metrics    *metrics.Service
	runtime    *runtime.Manager
	registry   *registry.Service
	catalog    *nodecapacity.Catalog
	placement  *placement.Engine
	local      nodeid.Local
	raft       *raftsvc.Service
	dispatcher WorkerDispatcher

	reconcileMu     sync.Mutex
	reconcileCancel context.CancelFunc
	reconcileRun    uint64
	reconcileWG     sync.WaitGroup
}

func NewService(logger *slog.Logger, metricService *metrics.Service, runtimeManager *runtime.Manager, registryService *registry.Service, cfg config.Config, bundle Bundle) *Service {
	return &Service{
		logger:     logger,
		cfg:        cfg,
		metrics:    metricService,
		runtime:    runtimeManager,
		registry:   registryService,
		catalog:    bundle.Catalog,
		placement:  bundle.Placement,
		local:      bundle.LocalNode,
		raft:       bundle.Raft,
		dispatcher: bundle.Dispatcher,
	}
}
