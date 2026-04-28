package nodeid

import (
	"context"
	"strings"
	"testing"

	"github.com/daiyuang/orch/internal/config"
)

func TestResolve_explicitOverridesHardware(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Raft.Node.ID = "  fixed-node  "
	got, err := Resolve(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "fixed-node" {
		t.Fatalf("got %q want fixed-node", got.Value)
	}
}

func TestResolve_autoKeywordUsesHardware(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Raft.Node.ID = "AuTo"
	got, err := Resolve(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.Value) == "" {
		t.Fatal("expected non-empty hardware/fallback id")
	}
	if got.Value == "AuTo" {
		t.Fatal("auto should not be used as literal id")
	}
}

func TestResolve_emptyMeansHardware(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Raft.Node.ID = ""
	got, err := Resolve(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value == "" {
		t.Fatal("expected resolved id")
	}
}
