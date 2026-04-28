package nodeid

import (
	"context"

	"github.com/daiyuang/orch/internal/config"
)

// New resolves [Local] at graph construction time using background context (same semantics as startup).
func New(cfg config.Config) (Local, error) {
	return Resolve(context.Background(), cfg)
}
