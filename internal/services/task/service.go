package task

import (
	"context"
	"log/slog"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type Service struct {
	logger    *slog.Logger
	cfg       config.Config
	metrics   *metrics.Service
	runtime   *runtime.Manager
	registry  *registry.Service
	catalog   *nodecapacity.Catalog
	placement *placement.Engine
	local     nodeid.Local
	raft      *raftsvc.Service
}

func NewService(logger *slog.Logger, metricService *metrics.Service, runtimeManager *runtime.Manager, registryService *registry.Service, cfg config.Config, bundle Bundle) *Service {
	return &Service{
		logger:    logger,
		cfg:       cfg,
		metrics:   metricService,
		runtime:   runtimeManager,
		registry:  registryService,
		catalog:   bundle.Catalog,
		placement: bundle.Placement,
		local:     bundle.LocalNode,
		raft:      bundle.Raft,
	}
}

// StartDeployReconcile runs a background loop that executes [Service.deployAppWorkloads] whenever the Raft FSM
// updates desired deploy documents (and once immediately for startup catch-up).
func (s *Service) StartDeployReconcile(ctx context.Context) {
	if s == nil || s.raft == nil {
		return
	}
	ch := s.raft.DeployReconcileSignals()
	if ch == nil {
		return
	}
	go func() {
		s.reconcileAll(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				s.reconcileAll(ctx)
			}
		}
	}()
}

func (s *Service) reconcileAll(ctx context.Context) {
	for _, app := range s.raft.ListDesiredDeployApps() {
		app := app
		if err := s.deployAppWorkloads(ctx, &app); err != nil {
			s.logger.Warn("deploy reconcile", "error", err, "app", app.Metadata.Name)
		}
	}
}

// SubmitDeploy validates the app and appends it to the replicated desired state (Raft when enabled).
// Local container startup happens asynchronously via [Service.StartDeployReconcile] on each node.
func (s *Service) SubmitDeploy(ctx context.Context, app *deployv1.App) error {
	if app == nil {
		return oopsx.B("task").Errorf("nil app")
	}
	if err := app.Validate(); err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return oopsx.B("task").Wrapf(err, "validate app")
	}
	if s.raft == nil {
		return oopsx.B("task").Errorf("raft service unavailable")
	}
	if err := s.raft.ApplyDeployApp(*app); err != nil {
		return oopsx.B("task").Wrapf(err, "replicate deploy")
	}
	s.logger.Info("deploy submitted", "app", app.Metadata.Name, "workloads", len(app.Workloads))
	return nil
}

// DeployApp is an alias for [Service.SubmitDeploy] (HTTP handlers and older callers).
func (s *Service) DeployApp(ctx context.Context, app *deployv1.App) error {
	return s.SubmitDeploy(ctx, app)
}

// deployAppWorkloads runs placement against the node resource catalog, then deploys each workload on the local
// runtime when placement selects this node (remote execution is not implemented yet).
func (s *Service) deployAppWorkloads(ctx context.Context, app *deployv1.App) error {
	if err := app.Validate(); err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return oopsx.B("task").Wrapf(err, "validate app")
	}

	if err := s.catalog.RefreshLocal(ctx, s.local, s.cfg); err != nil {
		s.logger.Warn("refresh local node capacity before placement", "error", err)
	}
	self := s.local.String()

	for i := range app.Workloads {
		w := &app.Workloads[i]
		chosen, err := s.placement.Choose(ctx, *w, s.catalog, self)
		if err != nil {
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			return oopsx.B("task").Wrapf(err, "placement workload %s", w.Name)
		}
		if chosen != self {
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			return oopsx.B("task").Errorf(
				"placement selected node %q for workload %q but remote execution is not implemented; add that node to the cluster catalog with live resource snapshots and run a worker orch-server there, or tighten resources / preferredNodes",
				chosen, w.Name,
			)
		}

		if err := s.runtime.Deploy(ctx, app.Metadata, *w); err != nil {
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			return oopsx.B("task").Wrapf(err, "deploy workload %s", w.Name)
		}
		s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "success")
		s.registry.Upsert(registry.WorkloadRecord{
			Name:    w.Name,
			Node:    chosen,
			Runtime: string(w.Runtime),
			Image:   w.Run.Image,
			Status:  "running",
		})
	}
	s.metrics.IncDeployApp(ctx, "success")
	s.logger.Info("application deployed", "app", app.Metadata.Name, "workloads", len(app.Workloads))
	return nil
}
