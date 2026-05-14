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
	if assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workload.Name)); ok {
		nodeID := strings.TrimSpace(assignment.Node)
		if nodeID != "" && nodeID != s.local.String() {
			return runtime.Status{
				Name:      workload.Name,
				Runtime:   workload.Runtime,
				Status:    nonEmptyStatus(assignment.Status, AppStatusPending),
				UpdatedAt: assignment.UpdatedAt,
				Message:   "workload is assigned to remote node " + nodeID + "; query that node for runtime-local status",
			}, nil
		}
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
	if assignment, ok := s.raft.GetWorkloadAssignment(workloadmeta.AssignmentKey(meta, workload.Name)); ok {
		nodeID := strings.TrimSpace(assignment.Node)
		if nodeID != "" && nodeID != s.local.String() {
			return runtime.LogResult{}, oopsx.B("task").Errorf("workload %q is assigned to remote node %q; query that node for logs", workload.Name, nodeID)
		}
	}
	out, err := s.runtime.Logs(ctx, workload.Runtime, meta, workload.Name, opts)
	if err != nil {
		return runtime.LogResult{}, oopsx.B("task").Wrapf(err, "read workload runtime logs")
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
