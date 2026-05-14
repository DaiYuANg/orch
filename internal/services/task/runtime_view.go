package task

import (
	"context"
	"strings"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (s *Service) WorkloadRuntimeStatus(ctx context.Context, meta deployv1.Metadata, workloadName string) (runtime.Status, error) {
	workload, ok := s.DesiredWorkload(meta, workloadName)
	if !ok {
		return runtime.Status{}, oopsx.B("task").Errorf("workload %s/%s/%s not found", meta.Namespace, meta.Name, workloadName)
	}
	if s.runtime == nil {
		return runtime.Status{}, oopsx.B("task").Errorf("runtime manager unavailable")
	}
	assignment, nodeID, ok := s.remoteAssignment(meta, workload.Name)
	if ok {
		return s.remoteWorkloadRuntimeStatus(ctx, meta, workload, assignment, nodeID)
	}
	out, err := s.runtime.Status(ctx, workload.Runtime, meta, workload.Name)
	if err != nil {
		return runtime.Status{}, oopsx.B("task").Wrapf(err, "read workload runtime status")
	}
	return out, nil
}

func (s *Service) WorkloadRuntimeLogs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts runtime.LogOptions) (runtime.LogResult, error) {
	workload, ok := s.DesiredWorkload(meta, workloadName)
	if !ok {
		return runtime.LogResult{}, oopsx.B("task").Errorf("workload %s/%s/%s not found", meta.Namespace, meta.Name, workloadName)
	}
	if s.runtime == nil {
		return runtime.LogResult{}, oopsx.B("task").Errorf("runtime manager unavailable")
	}
	_, nodeID, ok := s.remoteAssignment(meta, workload.Name)
	if ok {
		return s.remoteWorkloadRuntimeLogs(ctx, meta, workload, nodeID, opts)
	}
	out, err := s.runtime.Logs(ctx, workload.Runtime, meta, workload.Name, opts)
	if err != nil {
		return runtime.LogResult{}, oopsx.B("task").Wrapf(err, "read workload runtime logs")
	}
	return out, nil
}

func (s *Service) remoteAssignment(meta deployv1.Metadata, workloadName string) (workloadmeta.Assignment, string, bool) {
	if s == nil || s.raft == nil {
		return workloadmeta.Assignment{}, "", false
	}
	assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workloadName))
	if !ok {
		return workloadmeta.Assignment{}, "", false
	}
	nodeID := strings.TrimSpace(assignment.Node)
	return assignment, nodeID, nodeID != "" && nodeID != s.local.String()
}

func (s *Service) remoteWorkloadRuntimeStatus(
	ctx context.Context,
	meta deployv1.Metadata,
	workload deployv1.Workload,
	assignment workloadmeta.Assignment,
	nodeID string,
) (runtime.Status, error) {
	if s.dispatcher == nil {
		return runtime.Status{
			Name:      workload.Name,
			Runtime:   workload.Runtime,
			Status:    nonEmptyStatus(assignment.Status, AppStatusPending),
			UpdatedAt: assignment.UpdatedAt,
			Message:   "workload is assigned to remote node " + nodeID + " but worker dispatcher is unavailable",
		}, nil
	}
	out, err := s.dispatcher.WorkloadStatus(ctx, nodeID, meta, workload)
	if err != nil {
		return runtime.Status{}, oopsx.B("task").Wrapf(err, "read remote workload runtime status")
	}
	return out, nil
}

func (s *Service) remoteWorkloadRuntimeLogs(
	ctx context.Context,
	meta deployv1.Metadata,
	workload deployv1.Workload,
	nodeID string,
	opts runtime.LogOptions,
) (runtime.LogResult, error) {
	if s.dispatcher == nil {
		return runtime.LogResult{}, oopsx.B("task").Errorf("workload %q is assigned to remote node %q but worker dispatcher is unavailable", workload.Name, nodeID)
	}
	out, err := s.dispatcher.WorkloadLogs(ctx, nodeID, meta, workload, opts)
	if err != nil {
		return runtime.LogResult{}, oopsx.B("task").Wrapf(err, "read remote workload runtime logs")
	}
	return out, nil
}

func nonEmptyStatus(status, fallback string) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	return fallback
}
