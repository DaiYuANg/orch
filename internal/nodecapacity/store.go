package nodecapacity

import (
	"context"

	"github.com/arcgolabs/collectionx/list"
)

// SnapshotStore is the persistence backend for node resource snapshots (Raft-backed in cluster mode).
type SnapshotStore interface {
	Upsert(ctx context.Context, snap Snapshot) error
	Get(nodeID string) (Snapshot, bool)
	NodeIDs() *list.List[string]
	Len() int
}
