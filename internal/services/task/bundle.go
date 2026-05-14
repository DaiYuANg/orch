package task

import (
	"github.com/lyonbrown4d/orch/internal/nodecapacity"
	"github.com/lyonbrown4d/orch/internal/nodeid"
	"github.com/lyonbrown4d/orch/internal/placement"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
)

// Bundle groups singletons injected alongside core task dependencies (catalog + placement engine).
type Bundle struct {
	LocalNode  nodeid.Local
	Catalog    *nodecapacity.Catalog
	Placement  *placement.Engine
	Raft       *raftsvc.Service
	Dispatcher WorkerDispatcher
}

// NewBundle wires catalog, placement engine, resolved node id, and raft for deploy replication.
func NewBundle(local nodeid.Local, cat *nodecapacity.Catalog, eng *placement.Engine, rs *raftsvc.Service, dispatcher WorkerDispatcher) Bundle {
	return Bundle{LocalNode: local, Catalog: cat, Placement: eng, Raft: rs, Dispatcher: dispatcher}
}
