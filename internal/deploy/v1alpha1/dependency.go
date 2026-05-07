package v1alpha1

import (
	"errors"
	"strings"

	"github.com/arcgolabs/collectionx/graph"
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// WorkloadsInDependencyOrder returns workload copies ordered so dependencies appear before dependents.
func (a *App) WorkloadsInDependencyOrder() (*list.List[Workload], error) {
	workloads := a.WorkloadList()
	if workloads.Len() == 0 {
		return list.NewList[Workload](), nil
	}

	depGraph := graph.NewDirectedGraph[string, int]()
	seen := set.NewSetWithCapacity[string](workloads.Len())
	var buildErr error
	workloads.Range(func(i int, workload Workload) bool {
		name := strings.TrimSpace(workload.Name)
		if name == "" {
			buildErr = oopsx.B("deploy").Errorf("workloads[%d].name is required", i)
			return false
		}
		if seen.Contains(name) {
			buildErr = oopsx.B("deploy").Errorf("duplicate workload name %q", name)
			return false
		}
		seen.Add(name)
		depGraph.AddNode(name, i)
		return true
	})
	if buildErr != nil {
		return nil, buildErr
	}

	workloads.Range(func(i int, workload Workload) bool {
		workloadName := strings.TrimSpace(workload.Name)
		workload.DependsOnList().Range(func(j int, ref WorkloadRef) bool {
			depName := strings.TrimSpace(ref.Name)
			if !depGraph.HasNode(depName) {
				buildErr = oopsx.B("deploy").Errorf("workloads[%d].dependsOn[%d]: unknown workload %q", i, j, depName)
				return false
			}
			if err := depGraph.AddEdge(depName, workloadName); err != nil {
				buildErr = oopsx.B("deploy").Wrapf(err, "workloads[%d].dependsOn[%d]", i, j)
				return false
			}
			return true
		})
		return buildErr == nil
	})
	if buildErr != nil {
		return nil, buildErr
	}

	ids, err := depGraph.TopologicalSort()
	if err != nil {
		if errors.Is(err, graph.ErrCycleDetected) {
			return nil, oopsx.B("deploy").Errorf("workloads dependsOn contains a cycle")
		}
		return nil, oopsx.B("deploy").Wrapf(err, "workloads dependsOn graph")
	}

	out := list.NewListWithCapacity[Workload](len(ids))
	for _, id := range ids {
		idx, ok := depGraph.GetNode(id)
		if !ok {
			continue
		}
		workload, ok := workloads.Get(idx)
		if ok {
			out.Add(workload)
		}
	}
	return out, nil
}
