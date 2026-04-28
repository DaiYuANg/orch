package nodecapacity

import (
	"context"
	"strings"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/hostinfo"
	"github.com/daiyuang/orch/internal/nodeid"
)

// Catalog exposes placement-facing reads and RefreshLocal writes via [SnapshotStore].
type Catalog struct {
	store SnapshotStore
}

// NewCatalog wraps a [SnapshotStore] (production: [raftsvc.NewRaftCapacityStore]).
func NewCatalog(store SnapshotStore) *Catalog {
	if store == nil {
		panic("nodecapacity: SnapshotStore is required")
	}
	return &Catalog{store: store}
}

// Get returns the last snapshot for a node, if present.
func (c *Catalog) Get(nodeID string) (Snapshot, bool) {
	if c == nil || c.store == nil {
		return Snapshot{}, false
	}
	return c.store.Get(strings.TrimSpace(nodeID))
}

// Len returns how many nodes are tracked.
func (c *Catalog) Len() int {
	if c == nil || c.store == nil {
		return 0
	}
	return c.store.Len()
}

// NodeIDs returns sorted node ids for deterministic iteration.
func (c *Catalog) NodeIDs() []string {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.NodeIDs()
}

// RefreshLocal samples this host via hostinfo and upserts local node id through the store.
func (c *Catalog) RefreshLocal(ctx context.Context, local nodeid.Local, cfg config.Config) error {
	if c == nil || c.store == nil {
		return nil
	}
	nodeID := strings.TrimSpace(local.String())
	if nodeID == "" {
		nodeID = "local"
	}
	rep, err := hostinfo.Collect(ctx)
	if err != nil {
		return err
	}
	snap := snapshotFromReport(nodeID, rep)
	return c.store.Upsert(ctx, snap)
}
