package task

import (
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
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
	if wanted.IsEmpty() {
		return ordered, nil
	}
	selected := selectedOperationWorkloads(ordered, wanted)
	if !wanted.IsEmpty() {
		return nil, missingWorkloadError(wanted)
	}
	return selected, nil
}

func requestedWorkloadNames(names []string) *set.OrderedSet[string] {
	wanted := set.NewOrderedSetWithCapacity[string](len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name != "" {
			wanted.Add(name)
		}
	}
	return wanted
}

func selectedOperationWorkloads(ordered *list.List[deployv1.Workload], wanted *set.OrderedSet[string]) *list.List[deployv1.Workload] {
	selected := list.NewListWithCapacity[deployv1.Workload](wanted.Len())
	ordered.Range(func(_ int, workload deployv1.Workload) bool {
		name := strings.TrimSpace(workload.Name)
		if !wanted.Remove(name) {
			return true
		}
		selected.Add(workload)
		return true
	})
	return selected
}

func missingWorkloadError(wanted *set.OrderedSet[string]) error {
	missing := wanted.Values()
	if len(missing) > 1 {
		sort.Strings(missing)
	}
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
