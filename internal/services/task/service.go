package task

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
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
	s.raft.ListDesiredDeployApps().Range(func(_ int, app deployv1.App) bool {
		current := app
		if err := s.deployAppWorkloads(ctx, &current); err != nil {
			s.logger.Warn("deploy reconcile", "error", err, "app", current.Metadata.Name)
		}
		return true
	})
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

// deployAppWorkloads runs placement against the node resource catalog, then deploys locally or dispatches to a
// configured worker API when placement selects a remote node.
func (s *Service) deployAppWorkloads(ctx context.Context, app *deployv1.App) error {
	if err := app.Validate(); err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return oopsx.B("task").Wrapf(err, "validate app")
	}

	if err := s.catalog.RefreshLocal(ctx, s.local, s.cfg); err != nil {
		s.logger.Warn("refresh local node capacity before placement", "error", err)
	}
	self := s.local.String()
	workloads, err := app.WorkloadsInDependencyOrder()
	if err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return oopsx.B("task").Wrapf(err, "order workloads")
	}

	var deployErr error
	workloads.Range(func(_ int, w deployv1.Workload) bool {
		chosen, err := s.placement.Choose(ctx, w, s.catalog, self)
		if err != nil {
			s.applyWorkloadAssignment(app.Metadata, w, "", workloadmeta.AssignmentStatusFailed, err.Error())
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			deployErr = oopsx.B("task").Wrapf(err, "placement workload %s", w.Name)
			return false
		}
		s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusAssigned, "")
		if chosen != self {
			status, err := s.dispatchWorkload(ctx, app.Metadata, w, chosen)
			if err != nil {
				s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusFailed, err.Error())
				s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
				s.metrics.IncDeployApp(ctx, "failed")
				deployErr = err
				return false
			}
			if status == "" {
				status = "dispatched"
			}
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), status)
			s.registry.Upsert(registry.WorkloadRecord{
				Name:     w.Name,
				Node:     chosen,
				Runtime:  string(w.Runtime),
				Artifact: runconfig.ArtifactSummary(w.Run),
				Status:   status,
			})
			s.applyWorkloadAssignment(app.Metadata, w, chosen, status, "")
			return true
		}

		if err := s.deployLocalWorkload(ctx, app.Metadata, w, chosen); err != nil {
			s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusFailed, err.Error())
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			deployErr = err
			return false
		}
		s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusRunning, "")
		s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "success")
		return true
	})
	if deployErr != nil {
		return deployErr
	}
	s.metrics.IncDeployApp(ctx, "success")
	s.logger.Info("application deployed", "app", app.Metadata.Name, "workloads", len(app.Workloads))
	return nil
}

func (s *Service) applyWorkloadAssignment(meta deployv1.Metadata, workload deployv1.Workload, nodeID, status, errMsg string) {
	if s == nil || s.raft == nil {
		return
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = workloadmeta.AssignmentStatusAssigned
	}
	assignment := workloadmeta.Assignment{
		Key:       workloadmeta.AssignmentKey(meta, workload.Name),
		Metadata:  meta,
		Workload:  workload.Name,
		Node:      strings.TrimSpace(nodeID),
		Runtime:   workload.Runtime,
		Artifact:  runconfig.ArtifactSummary(workload.Run),
		Status:    status,
		Error:     strings.TrimSpace(errMsg),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.raft.ApplyWorkloadAssignment(assignment); err != nil {
		s.logger.Warn("workload assignment apply",
			"error", err,
			"app", meta.Name,
			"workload", workload.Name,
			"node", nodeID,
			"status", status,
		)
	}
}

func (s *Service) dispatchWorkload(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload, nodeID string) (string, error) {
	if s.dispatcher == nil {
		return "", oopsx.B("task").Errorf("placement selected node %q for workload %q but worker dispatcher is unavailable", nodeID, workload.Name)
	}
	result, err := s.dispatcher.DispatchWorkload(ctx, nodeID, meta, workload)
	if err != nil {
		return "", oopsx.B("task").Wrapf(err, "dispatch workload %s to node %s", workload.Name, nodeID)
	}
	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "dispatched"
	}
	s.logger.Info("workload dispatched", "workload", workload.Name, "node", nodeID, "runtime", workload.Runtime, "status", status)
	return status, nil
}

func (s *Service) deployLocalWorkload(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload, nodeID string) error {
	if err := s.runtime.Deploy(ctx, meta, workload); err != nil {
		return oopsx.B("task").Wrapf(err, "deploy workload %s", workload.Name)
	}
	s.registry.Upsert(registry.WorkloadRecord{
		Name:     workload.Name,
		Node:     nodeID,
		Runtime:  string(workload.Runtime),
		Artifact: runconfig.ArtifactSummary(workload.Run),
		Status:   "running",
	})
	return nil
}

// DeployWorkerWorkload executes a workload assigned by another node. It intentionally bypasses Raft desired-state
// mutation; callers must already have gone through SubmitDeploy on the scheduling node.
func (s *Service) DeployWorkerWorkload(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload, assignedNode string) error {
	if strings.TrimSpace(meta.Name) == "" {
		return oopsx.B("task", "worker").Errorf("metadata.name is required")
	}
	if strings.TrimSpace(workload.Name) == "" {
		return oopsx.B("task", "worker").Errorf("workload.name is required")
	}
	self := s.local.String()
	if n := strings.TrimSpace(assignedNode); n != "" && self != "" && n != self {
		return oopsx.B("task", "worker").Errorf("workload %q assigned to node %q, local node is %q", workload.Name, n, self)
	}
	if err := s.deployLocalWorkload(ctx, meta, workload, self); err != nil {
		s.metrics.IncDeployWorkload(ctx, string(workload.Runtime), "failed")
		return err
	}
	s.metrics.IncDeployWorkload(ctx, string(workload.Runtime), "success")
	return nil
}
