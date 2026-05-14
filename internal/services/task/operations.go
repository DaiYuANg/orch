package task

import (
	"context"
	"strings"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
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
	workloads, err := s.failoverWorkloads(app, opts.Workloads)
	if err != nil {
		return AppOperationSummary{}, err
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
	meta = normalizeOperationMetadata(meta)
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
