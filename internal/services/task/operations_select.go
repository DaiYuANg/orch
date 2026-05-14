package task

import (
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func (s *Service) failoverWorkloads(app *deployv1.App, names []string) (*list.List[deployv1.Workload], error) {
	workloads, err := selectOperationWorkloads(app, names)
	if err != nil {
		return nil, err
	}
	if len(names) > 0 {
		return workloads, nil
	}
	failed := s.failedWorkloads(app.Metadata, workloads)
	if failed.Len() == 0 {
		return nil, oopsx.B("task").Errorf("deploy app %s/%s has no failed workloads", workloadmeta.NamespaceOrDefault(app.Metadata.Namespace), app.Metadata.Name)
	}
	return failed, nil
}

func selectOperationWorkloads(app *deployv1.App, names []string) (*list.List[deployv1.Workload], error) {
	if app == nil {
		return list.NewList[deployv1.Workload](), nil
	}
	ordered, err := app.WorkloadsInDependencyOrder()
	if err != nil {
		return nil, oopsx.B("task").Wrapf(err, "order workloads")
	}
	wanted := requestedWorkloadNames(names)
	if len(wanted) == 0 {
		return ordered, nil
	}
	selected := selectedOperationWorkloads(ordered, wanted)
	if len(wanted) > 0 {
		return nil, missingWorkloadError(wanted)
	}
	return selected, nil
}

func requestedWorkloadNames(names []string) map[string]struct{} {
	wanted := make(map[string]struct{}, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name != "" {
			wanted[name] = struct{}{}
		}
	}
	return wanted
}

func selectedOperationWorkloads(ordered *list.List[deployv1.Workload], wanted map[string]struct{}) *list.List[deployv1.Workload] {
	selected := list.NewListWithCapacity[deployv1.Workload](len(wanted))
	ordered.Range(func(_ int, workload deployv1.Workload) bool {
		name := strings.TrimSpace(workload.Name)
		if _, ok := wanted[name]; !ok {
			return true
		}
		selected.Add(workload)
		delete(wanted, name)
		return true
	})
	return selected
}

func missingWorkloadError(wanted map[string]struct{}) error {
	missing := make([]string, 0, len(wanted))
	for name := range wanted {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return oopsx.B("task").Errorf("workload(s) not found: %s", strings.Join(missing, ", "))
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
