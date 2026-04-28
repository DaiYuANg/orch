package placement

import (
	"context"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodecapacity"
)

// Well-known placement algorithm identifiers (iterate new versions without breaking callers).
const (
	VersionV1Alpha1 = "v1alpha1"
)

// Engine is an injectable placement policy implementation; callers resolve workloads via Choose.
type Engine struct {
	// Version selects the algorithm revision (defaults to VersionV1Alpha1).
	Version string
}

// NewEngine returns the default placement engine (currently v1alpha1 scoring).
func NewEngine() *Engine {
	return &Engine{Version: VersionV1Alpha1}
}

func (e *Engine) versionKey() string {
	if v := e.Version; v != "" {
		return v
	}
	return VersionV1Alpha1
}

// Choose selects the best node id for w using catalog snapshots.
func (e *Engine) Choose(ctx context.Context, w deployv1.Workload, catalog *nodecapacity.Catalog, localNodeID string) (string, error) {
	_ = e.versionKey() // dispatch point for future algorithm revisions (v1beta1, …)
	return chooseV1Alpha1(ctx, w, catalog, localNodeID)
}
