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

func (s *Service) SubmitDelete(ctx context.Context, meta deployv1.Metadata) error {
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Namespace = strings.TrimSpace(meta.Namespace)
	if meta.Name == "" {
		return oopsx.B("task").Errorf("metadata.name is required")
	}
	if s.raft == nil {
		return oopsx.B("task").Errorf("raft service unavailable")
	}
	app, ok := s.raft.GetDesiredDeployApp(meta)
	if ok {
		if err := s.stopAppWorkloads(ctx, &app); err != nil {
			return err
		}
	}
	if err := s.raft.ApplyDeleteDeployApp(meta); err != nil {
		return oopsx.B("task").Wrapf(err, "delete desired app")
	}
	s.logger.Info("deploy deleted", "app", meta.Name, "namespace", workloadmeta.NamespaceOrDefault(meta.Namespace))
	return nil
}

func (s *Service) SubmitStop(ctx context.Context, meta deployv1.Metadata) error {
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Namespace = strings.TrimSpace(meta.Namespace)
	if meta.Name == "" {
		return oopsx.B("task").Errorf("metadata.name is required")
	}
	if s.raft == nil {
		return oopsx.B("task").Errorf("raft service unavailable")
	}
	app, ok := s.raft.GetDesiredDeployApp(meta)
	if !ok {
		return oopsx.B("task").Errorf("deploy app %s/%s not found", workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name)
	}
	if err := s.stopAppWorkloads(ctx, &app); err != nil {
		return err
	}
	s.logger.Info("deploy stopped", "app", meta.Name, "namespace", workloadmeta.NamespaceOrDefault(meta.Namespace))
	return nil
}

func (s *Service) SubmitStart(ctx context.Context, meta deployv1.Metadata) error {
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Namespace = strings.TrimSpace(meta.Namespace)
	if meta.Name == "" {
		return oopsx.B("task").Errorf("metadata.name is required")
	}
	if s.raft == nil {
		return oopsx.B("task").Errorf("raft service unavailable")
	}
	app, ok := s.raft.GetDesiredDeployApp(meta)
	if !ok {
		return oopsx.B("task").Errorf("deploy app %s/%s not found", workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name)
	}
	if err := s.deployAppWorkloads(ctx, &app); err != nil {
		return err
	}
	s.logger.Info("deploy started", "app", meta.Name, "namespace", workloadmeta.NamespaceOrDefault(meta.Namespace))
	return nil
}

func (s *Service) SubmitRestart(ctx context.Context, meta deployv1.Metadata) error {
	if err := s.SubmitStop(ctx, meta); err != nil {
		return err
	}
	if err := s.SubmitStart(ctx, meta); err != nil {
		return err
	}
	s.logger.Info("deploy restarted", "app", meta.Name, "namespace", workloadmeta.NamespaceOrDefault(meta.Namespace))
	return nil
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
	generation := AppGeneration(*app)
	workloads, err := app.WorkloadsInDependencyOrder()
	if err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return oopsx.B("task").Wrapf(err, "order workloads")
	}

	var deployErr error
	workloads.Range(func(_ int, w deployv1.Workload) bool {
		chosen, err := s.chooseWorkloadNode(ctx, w, self)
		if err != nil {
			s.applyWorkloadAssignment(app.Metadata, w, "", workloadmeta.AssignmentStatusFailed, generation, err.Error())
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			deployErr = oopsx.B("task").Wrapf(err, "placement workload %s", w.Name)
			return false
		}
		s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusAssigned, generation, "")
		if chosen != self {
			status, err := s.dispatchWorkload(ctx, app.Metadata, w, chosen)
			if err != nil {
				s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusFailed, generation, err.Error())
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
			s.applyWorkloadAssignment(app.Metadata, w, chosen, status, generation, "")
			return true
		}

		if err := s.deployLocalWorkload(ctx, app.Metadata, w, chosen); err != nil {
			s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusFailed, generation, err.Error())
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			deployErr = err
			return false
		}
		s.applyWorkloadAssignment(app.Metadata, w, chosen, workloadmeta.AssignmentStatusRunning, generation, "")
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

func (s *Service) stopAppWorkloads(ctx context.Context, app *deployv1.App) error {
	if app == nil {
		return nil
	}
	workloads, err := app.WorkloadsInDependencyOrder()
	if err != nil {
		return oopsx.B("task").Wrapf(err, "order workloads")
	}
	generation := AppGeneration(*app)
	values := workloads.Values()
	for i := len(values) - 1; i >= 0; i-- {
		w := values[i]
		if err := s.stopWorkload(ctx, app.Metadata, w); err != nil {
			s.applyWorkloadAssignment(app.Metadata, w, assignmentNodeOrEmpty(s.raft, app.Metadata, w.Name), workloadmeta.AssignmentStatusFailed, generation, err.Error())
			return err
		}
		s.applyWorkloadAssignment(app.Metadata, w, assignmentNodeOrEmpty(s.raft, app.Metadata, w.Name), workloadmeta.AssignmentStatusStopped, generation, "")
	}
	return nil
}

func (s *Service) chooseWorkloadNode(ctx context.Context, workload deployv1.Workload, self string) (string, error) {
	chosen, err := s.placement.Choose(ctx, workload, s.catalog, self)
	if err == nil {
		return chosen, nil
	}
	fallback, ok := s.preferredConfiguredNode(workload, self)
	if !ok {
		return "", err
	}
	s.logger.Warn("placement fallback to configured preferred node without capacity snapshot",
		"node", fallback,
		"workload", workload.Name,
		"error", err,
	)
	return fallback, nil
}

func (s *Service) preferredConfiguredNode(workload deployv1.Workload, self string) (string, bool) {
	if s == nil || workload.Scheduling == nil || len(workload.Scheduling.PreferredNodes) == 0 {
		return "", false
	}
	for _, raw := range workload.Scheduling.PreferredNodes {
		nodeID := strings.TrimSpace(raw)
		if nodeID == "" || nodeID == strings.TrimSpace(self) {
			continue
		}
		if _, ok := s.cfg.Cluster.NodeURL(nodeID); ok {
			return nodeID, true
		}
	}
	return "", false
}

func assignmentNodeOrEmpty(raft *raftsvc.Service, meta deployv1.Metadata, workloadName string) string {
	if raft == nil {
		return ""
	}
	assignment, ok := raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workloadName))
	if !ok {
		return ""
	}
	return assignment.Node
}

func (s *Service) stopWorkload(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload) error {
	key := workloadmeta.AssignmentKey(meta, workload.Name)
	assignment, ok := s.raft.GetWorkloadAssignment(key)
	nodeID := s.local.String()
	if ok && strings.TrimSpace(assignment.Node) != "" {
		nodeID = strings.TrimSpace(assignment.Node)
	}
	if nodeID != "" && nodeID != s.local.String() {
		if s.dispatcher == nil {
			return oopsx.B("task").Errorf("workload %q assigned to remote node %q but worker dispatcher is unavailable", workload.Name, nodeID)
		}
		result, err := s.dispatcher.StopWorkload(ctx, nodeID, meta, workload)
		if err != nil {
			return oopsx.B("task").Wrapf(err, "stop workload %s on node %s", workload.Name, nodeID)
		}
		status := strings.TrimSpace(result.Status)
		if status == "" {
			status = workloadmeta.AssignmentStatusStopped
		}
		s.registry.Delete(workload.Name)
		s.logger.Info("workload stop dispatched", "workload", workload.Name, "node", nodeID, "runtime", workload.Runtime, "status", status)
		return nil
	}
	if err := s.runtime.Stop(ctx, workload.Runtime, meta, workload.Name); err != nil {
		return oopsx.B("task").Wrapf(err, "stop workload %s", workload.Name)
	}
	s.registry.Delete(workload.Name)
	return nil
}

func (s *Service) applyWorkloadAssignment(meta deployv1.Metadata, workload deployv1.Workload, nodeID, status, generation, errMsg string) {
	if s == nil || s.raft == nil {
		return
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = workloadmeta.AssignmentStatusAssigned
	}
	assignment := workloadmeta.Assignment{
		Key:        workloadmeta.AssignmentKey(meta, workload.Name),
		Metadata:   meta,
		Workload:   workload.Name,
		Node:       strings.TrimSpace(nodeID),
		Runtime:    workload.Runtime,
		Artifact:   runconfig.ArtifactSummary(workload.Run),
		Status:     status,
		Generation: strings.TrimSpace(generation),
		Error:      strings.TrimSpace(errMsg),
		UpdatedAt:  time.Now().UTC(),
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

func (s *Service) StopWorkerWorkload(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload, assignedNode string) error {
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
	if err := s.runtime.Stop(ctx, workload.Runtime, meta, workload.Name); err != nil {
		s.metrics.IncDeployWorkload(ctx, string(workload.Runtime), "failed")
		return err
	}
	s.registry.Delete(workload.Name)
	s.metrics.IncDeployWorkload(ctx, string(workload.Runtime), workloadmeta.AssignmentStatusStopped)
	return nil
}
