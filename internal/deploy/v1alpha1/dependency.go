package v1alpha1

import (
	"errors"
	"strings"

	"github.com/arcgolabs/collectionx/graph"
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"

	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// WorkloadsInDependencyOrder returns workload copies ordered so dependencies appear before dependents.
func (a *App) WorkloadsInDependencyOrder() (*list.List[Workload], error) {
	workloads := a.WorkloadList()
	if workloads.Len() == 0 {
		return list.NewList[Workload](), nil
	}
	depGraph, err := buildWorkloadDependencyGraph(workloads)
	if err != nil {
		return nil, err
	}
	return orderedWorkloads(workloads, depGraph)
}

func buildWorkloadDependencyGraph(workloads *list.List[Workload]) (*graph.Graph[string, int], error) {
	depGraph := graph.NewDirectedGraph[string, int]()
	if err := addWorkloadDependencyNodes(workloads, depGraph); err != nil {
		return nil, err
	}
	if err := addWorkloadDependencyEdges(workloads, depGraph); err != nil {
		return nil, err
	}
	return depGraph, nil
}

func addWorkloadDependencyNodes(workloads *list.List[Workload], depGraph *graph.Graph[string, int]) error {
	seen := set.NewSetWithCapacity[string](workloads.Len())
	var buildErr error
	workloads.Range(func(i int, workload Workload) bool {
		buildErr = addWorkloadDependencyNode(i, workload, depGraph, seen)
		return buildErr == nil
	})
	return buildErr
}

func addWorkloadDependencyNode(i int, workload Workload, depGraph *graph.Graph[string, int], seen *set.Set[string]) error {
	name := strings.TrimSpace(workload.Name)
	if name == "" {
		return oopsx.B("deploy").Errorf("workloads[%d].name is required", i)
	}
	if seen.Contains(name) {
		return oopsx.B("deploy").Errorf("duplicate workload name %q", name)
	}
	seen.Add(name)
	depGraph.AddNode(name, i)
	return nil
}

func addWorkloadDependencyEdges(workloads *list.List[Workload], depGraph *graph.Graph[string, int]) error {
	var buildErr error
	workloads.Range(func(i int, workload Workload) bool {
		buildErr = addWorkloadDependencyEdgesForWorkload(i, workload, depGraph)
		return buildErr == nil
	})
	return buildErr
}

func addWorkloadDependencyEdgesForWorkload(i int, workload Workload, depGraph *graph.Graph[string, int]) error {
	var buildErr error
	workloadName := strings.TrimSpace(workload.Name)
	workload.DependsOnList().Range(func(j int, ref WorkloadRef) bool {
		buildErr = addWorkloadDependencyEdge(i, j, strings.TrimSpace(ref.Name), workloadName, depGraph)
		return buildErr == nil
	})
	return buildErr
}

func addWorkloadDependencyEdge(i, j int, depName, workloadName string, depGraph *graph.Graph[string, int]) error {
	if !depGraph.HasNode(depName) {
		return oopsx.B("deploy").Errorf("workloads[%d].dependsOn[%d]: unknown workload %q", i, j, depName)
	}
	if err := depGraph.AddEdge(depName, workloadName); err != nil {
		return oopsx.B("deploy").Wrapf(err, "workloads[%d].dependsOn[%d]", i, j)
	}
	return nil
}

func orderedWorkloads(workloads *list.List[Workload], depGraph *graph.Graph[string, int]) (*list.List[Workload], error) {
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
