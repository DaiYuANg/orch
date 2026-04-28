package nodeid

import (
	"context"
	"fmt"
	"strings"

	"github.com/daiyuang/orch/internal/config"
)

// Local is the resolved stable node identifier for this process (Os host id unless overridden via config).
type Local struct {
	// Value is non-empty after successful resolution (Raft LocalID, placement self, catalog snapshots).
	Value string
}

func (l Local) String() string {
	return l.Value
}

// Resolve chooses the node id: non-empty raft.node.id (not "auto") wins; otherwise OS hardware id
// (see [FromHardware]) with a deterministic network fallback when the OS does not expose one.
func Resolve(ctx context.Context, cfg config.Config) (Local, error) {
	explicit := strings.TrimSpace(cfg.Raft.Node.ID)
	if explicit != "" && !strings.EqualFold(explicit, "auto") {
		return Local{Value: explicit}, nil
	}
	v, err := FromHardware(ctx)
	if err != nil {
		return Local{}, err
	}
	if strings.TrimSpace(v) == "" {
		return Local{}, fmt.Errorf("nodeid: hardware id resolved empty")
	}
	return Local{Value: v}, nil
}
