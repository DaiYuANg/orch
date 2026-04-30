package task

import (
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
)

// Bundle groups singletons injected alongside core task dependencies (catalog + placement engine).
type Bundle struct {
	LocalNode nodeid.Local
	Catalog   *nodecapacity.Catalog
	Placement *placement.Engine
	Raft      *raftsvc.Service
}

// NewBundle wires catalog, placement engine, resolved node id, and raft for deploy replication.
func NewBundle(local nodeid.Local, cat *nodecapacity.Catalog, eng *placement.Engine, rs *raftsvc.Service) Bundle {
	return Bundle{LocalNode: local, Catalog: cat, Placement: eng, Raft: rs}
}
