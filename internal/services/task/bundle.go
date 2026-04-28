package task

import (
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/placement"
)

// Bundle groups singletons injected alongside core task dependencies (catalog + placement engine).
type Bundle struct {
	LocalNode nodeid.Local
	Catalog   *nodecapacity.Catalog
	Placement *placement.Engine
}

// NewBundle wires catalog, placement engine, and resolved node id for composition roots (e.g. dix.Provider3).
func NewBundle(local nodeid.Local, cat *nodecapacity.Catalog, eng *placement.Engine) Bundle {
	return Bundle{LocalNode: local, Catalog: cat, Placement: eng}
}
