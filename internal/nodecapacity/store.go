package nodecapacity

import (
	"context"
)

// SnapshotStore is the persistence backend for node resource snapshots (Raft-backed in cluster mode).
type SnapshotStore interface {
	Upsert(ctx context.Context, snap Snapshot) error
	Get(nodeID string) (Snapshot, bool)
	NodeIDs() []string
	Len() int
}
