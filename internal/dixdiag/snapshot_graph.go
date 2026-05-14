package dixdiag

import (
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"
)

func graphSnapshot(rt *dix.Runtime) GraphSnapshot {
	graph, err := rt.DependencyGraph()
	out := GraphSnapshot{}
	if err != nil {
		out.Error = err.Error()
	}
	addGraphNodes(&out, graph.Nodes)
	if graph.Edges != nil {
		out.Edges = graph.Edges.Len()
	}
	return out
}

func addGraphNodes(out *GraphSnapshot, nodes *list.List[dix.DependencyGraphNode]) {
	if nodes == nil {
		return
	}
	out.Nodes = nodes.Len()
	nodes.Range(func(_ int, node dix.DependencyGraphNode) bool {
		addGraphNode(out, node)
		return true
	})
}

func addGraphNode(out *GraphSnapshot, node dix.DependencyGraphNode) {
	switch node.Kind {
	case dix.DependencyGraphNodeApp:
		out.Apps++
	case dix.DependencyGraphNodeModule:
		out.Modules++
	case dix.DependencyGraphNodeService:
		out.Services++
	case dix.DependencyGraphNodeOperation:
		addGraphOperationNode(out, node)
	}
}

func addGraphOperationNode(out *GraphSnapshot, node dix.DependencyGraphNode) {
	out.Operations++
	if node.Eager {
		out.EagerOperations++
	}
	if node.Raw {
		out.RawOperations++
	}
}
