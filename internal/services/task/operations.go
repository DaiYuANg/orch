package task

import (
	"context"
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	OperationMigrate   = "migrate"
	OperationFailover  = "failover"
	OperationRebalance = "rebalance"
)

type AppOperationOptions struct {
	TargetNode string
	Workloads  []string
}

type AppOperationSummary struct {
	Operation  string
	App        string
	Namespace  string
	TargetNode string
	Workloads  int
	Moved      int
	Status     string
}

type plannedWorkloadMove struct {
	workload deployv1.Workload
	current  string
	target   string
	assigned bool
}

func (s *Service) SubmitMigrate(ctx context.Context, meta deployv1.Metadata, opts AppOperationOptions) (AppOperationSummary, error) {
	target := strings.TrimSpace(opts.TargetNode)
	if target == "" {
		return AppOperationSummary{}, oopsx.B("task").Errorf("target node is required")
	}
	app, err := s.operationApp(meta)
	if err != nil {
		return AppOperationSummary{}, err
	}
	workloads, err := selectOperationWorkloads(app, opts.Workloads)
	if err != nil {
		return AppOperationSummary{}, err
	}
	return s.moveAppWorkloads(ctx, app, OperationMigrate, workloads, func(deployv1.Workload) (string, error) {
		return target, nil
	})
}

func (s *Service) SubmitFailover(ctx context.Context, meta deployv1.Metadata, opts AppOperationOptions) (AppOperationSummary, error) {
	app, err := s.operationApp(meta)
	if err != nil {
		return AppOperationSummary{}, err
	}
	workloads, err := selectOperationWorkloads(app, opts.Workloads)
	if err != nil {
		return AppOperationSummary{}, err
	}
	if len(opts.Workloads) == 0 {
		workloads = s.failedWorkloads(app.Metadata, workloads)
		if workloads.Len() == 0 {
			return AppOperationSummary{}, oopsx.B("task").Errorf("deploy app %s/%s has no failed workloads", workloadmeta.NamespaceOrDefault(app.Metadata.Namespace), app.Metadata.Name)
		}
	}
	target := strings.TrimSpace(opts.TargetNode)
	return s.moveAppWorkloads(ctx, app, OperationFailover, workloads, func(w deployv1.Workload) (string, error) {
		if target != "" {
			return target, nil
		}
		current := s.currentWorkloadNode(app.Metadata, w.Name)
		return s.chooseFailoverNode(ctx, w, current)
	})
}

func (s *Service) SubmitRebalance(ctx context.Context, meta deployv1.Metadata, opts AppOperationOptions) (AppOperationSummary, error) {
	app, err := s.operationApp(meta)
	if err != nil {
		return AppOperationSummary{}, err
	}
	workloads, err := selectOperationWorkloads(app, opts.Workloads)
	if err != nil {
		return AppOperationSummary{}, err
	}
	if err := s.catalog.RefreshLocal(ctx, s.local, s.cfg); err != nil {
		s.logger.Warn("refresh local node capacity before rebalance", "error", err)
	}
	self := s.local.String()
	return s.moveAppWorkloads(ctx, app, OperationRebalance, workloads, func(w deployv1.Workload) (string, error) {
		return s.chooseWorkloadNode(ctx, w, self)
	})
}

func (s *Service) operationApp(meta deployv1.Metadata) (*deployv1.App, error) {
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Namespace = strings.TrimSpace(meta.Namespace)
	if meta.Name == "" {
		return nil, oopsx.B("task").Errorf("metadata.name is required")
	}
	if s.raft == nil {
		return nil, oopsx.B("task").Errorf("raft service unavailable")
	}
	app, ok := s.raft.GetDesiredDeployApp(meta)
	if !ok {
		return nil, oopsx.B("task").Errorf("deploy app %s/%s not found", workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name)
	}
	return &app, nil
}

func selectOperationWorkloads(app *deployv1.App, names []string) (*list.List[deployv1.Workload], error) {
	if app == nil {
		return list.NewList[deployv1.Workload](), nil
	}
	ordered, err := app.WorkloadsInDependencyOrder()
	if err != nil {
		return nil, oopsx.B("task").Wrapf(err, "order workloads")
	}
	wanted := map[string]struct{}{}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return ordered, nil
	}
	selected := list.NewListWithCapacity[deployv1.Workload](len(wanted))
	ordered.Range(func(_ int, workload deployv1.Workload) bool {
		name := strings.TrimSpace(workload.Name)
		if _, ok := wanted[name]; ok {
			selected.Add(workload)
			delete(wanted, name)
		}
		return true
	})
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for name := range wanted {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, oopsx.B("task").Errorf("workload(s) not found: %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

func (s *Service) failedWorkloads(meta deployv1.Metadata, workloads *list.List[deployv1.Workload]) *list.List[deployv1.Workload] {
	out := list.NewList[deployv1.Workload]()
	workloads.Range(func(_ int, workload deployv1.Workload) bool {
		assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workload.Name))
		if ok && assignment.Status == workloadmeta.AssignmentStatusFailed {
			out.Add(workload)
		}
		return true
	})
	return out
}

func (s *Service) moveAppWorkloads(ctx context.Context, app *deployv1.App, operation string, workloads *list.List[deployv1.Workload], targetFor func(deployv1.Workload) (string, error)) (AppOperationSummary, error) {
	summary := AppOperationSummary{
		Operation: operation,
		App:       app.Metadata.Name,
		Namespace: workloadmeta.NamespaceOrDefault(app.Metadata.Namespace),
		Workloads: workloads.Len(),
		Status:    workloadmeta.AssignmentStatusRunning,
	}
	generation := AppGeneration(*app)
	self := s.local.String()
	plans := make([]plannedWorkloadMove, 0, workloads.Len())
	var selectErr error
	workloads.Range(func(_ int, workload deployv1.Workload) bool {
		target, err := targetFor(workload)
		if err != nil {
			selectErr = err
			summary.Status = workloadmeta.AssignmentStatusFailed
			return false
		}
		target = strings.TrimSpace(target)
		if target == "" {
			selectErr = oopsx.B("task").Errorf("empty target node for workload %q", workload.Name)
			summary.Status = workloadmeta.AssignmentStatusFailed
			return false
		}
		current, assigned := s.currentWorkloadAssignmentNode(app.Metadata, workload.Name)
		if current == "" {
			current = self
		}
		if assigned && current == target {
			return true
		}
		if summary.TargetNode == "" {
			summary.TargetNode = target
		}
		plans = append(plans, plannedWorkloadMove{workload: workload, current: current, target: target, assigned: assigned})
		return true
	})
	if summary.Status == workloadmeta.AssignmentStatusFailed {
		return summary, oopsx.B("task").Wrapf(selectErr, "%s target selection failed", operation)
	}
	if len(plans) == 0 {
		summary.Status = "unchanged"
		return summary, nil
	}

	for i := len(plans) - 1; i >= 0; i-- {
		plan := plans[i]
		if !plan.assigned {
			continue
		}
		if operation == OperationFailover && s.workloadAssignmentStatus(app.Metadata, plan.workload.Name) == workloadmeta.AssignmentStatusFailed {
			continue
		}
		if err := s.stopWorkload(ctx, app.Metadata, plan.workload); err != nil {
			s.applyWorkloadAssignment(app.Metadata, plan.workload, plan.current, workloadmeta.AssignmentStatusFailed, generation, err.Error())
			summary.Status = workloadmeta.AssignmentStatusFailed
			return summary, err
		}
		s.applyWorkloadAssignment(app.Metadata, plan.workload, plan.current, workloadmeta.AssignmentStatusStopped, generation, "")
	}

	for _, plan := range plans {
		s.applyWorkloadAssignment(app.Metadata, plan.workload, plan.target, workloadmeta.AssignmentStatusAssigned, generation, "")
		status, err := s.runWorkloadOnNode(ctx, app.Metadata, plan.workload, plan.target)
		if err != nil {
			s.applyWorkloadAssignment(app.Metadata, plan.workload, plan.target, workloadmeta.AssignmentStatusFailed, generation, err.Error())
			summary.Status = workloadmeta.AssignmentStatusFailed
			return summary, err
		}
		if status == "" {
			status = workloadmeta.AssignmentStatusRunning
		}
		s.applyWorkloadAssignment(app.Metadata, plan.workload, plan.target, status, generation, "")
		summary.Moved++
	}
	s.logger.Info("deploy operation completed",
		"operation", operation,
		"app", app.Metadata.Name,
		"namespace", workloadmeta.NamespaceOrDefault(app.Metadata.Namespace),
		"workloads", summary.Workloads,
		"moved", summary.Moved,
	)
	return summary, nil
}

func (s *Service) currentWorkloadNode(meta deployv1.Metadata, workloadName string) string {
	return assignmentNodeOrEmpty(s.raft, meta, workloadName)
}

func (s *Service) currentWorkloadAssignmentNode(meta deployv1.Metadata, workloadName string) (string, bool) {
	if s == nil || s.raft == nil {
		return "", false
	}
	assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workloadName))
	if !ok {
		return "", false
	}
	return strings.TrimSpace(assignment.Node), true
}

func (s *Service) workloadAssignmentStatus(meta deployv1.Metadata, workloadName string) string {
	if s == nil || s.raft == nil {
		return ""
	}
	assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workloadName))
	if !ok {
		return ""
	}
	return strings.TrimSpace(assignment.Status)
}

func (s *Service) chooseFailoverNode(ctx context.Context, workload deployv1.Workload, current string) (string, error) {
	if err := s.catalog.RefreshLocal(ctx, s.local, s.cfg); err != nil {
		s.logger.Warn("refresh local node capacity before failover", "error", err)
	}
	candidates := s.alternativeNodeCandidates(current)
	if candidates.Len() == 0 {
		return "", oopsx.B("task").Errorf("no alternative node available for workload %q", workload.Name)
	}
	candidate := workload
	scheduling := deployv1.Scheduling{}
	if workload.Scheduling != nil {
		scheduling = *workload.Scheduling
	}
	scheduling.PreferredNodes = candidates.Values()
	candidate.Scheduling = &scheduling
	return s.chooseWorkloadNode(ctx, candidate, s.local.String())
}

func (s *Service) alternativeNodeCandidates(current string) *list.List[string] {
	current = strings.TrimSpace(current)
	seen := map[string]struct{}{}
	out := list.NewList[string]()
	add := func(nodeID string) {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" || nodeID == current {
			return
		}
		if _, ok := seen[nodeID]; ok {
			return
		}
		seen[nodeID] = struct{}{}
		out.Add(nodeID)
	}
	s.catalog.NodeIDs().Range(func(_ int, nodeID string) bool {
		add(nodeID)
		return true
	})
	cfgNodes := make([]string, 0, len(s.cfg.Cluster.Nodes))
	for nodeID := range s.cfg.Cluster.Nodes {
		cfgNodes = append(cfgNodes, nodeID)
	}
	sort.Strings(cfgNodes)
	for _, nodeID := range cfgNodes {
		add(nodeID)
	}
	add(s.local.String())
	return out
}

func (s *Service) runWorkloadOnNode(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload, nodeID string) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return "", oopsx.B("task").Errorf("empty target node for workload %q", workload.Name)
	}
	if nodeID != s.local.String() {
		status, err := s.dispatchWorkload(ctx, meta, workload, nodeID)
		if err != nil {
			return "", err
		}
		if status == "" {
			status = "dispatched"
		}
		s.registry.Upsert(registry.WorkloadRecord{
			Name:     workload.Name,
			Node:     nodeID,
			Runtime:  string(workload.Runtime),
			Artifact: runconfig.ArtifactSummary(workload.Run),
			Status:   status,
		})
		return status, nil
	}
	if err := s.deployLocalWorkload(ctx, meta, workload, nodeID); err != nil {
		return "", err
	}
	return workloadmeta.AssignmentStatusRunning, nil
}
