package gossipsvc_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/gossipsvc"
	"github.com/lyonbrown4d/orch/internal/nodeid"
)

func TestDisabledServiceStartStop(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	svc := gossipsvc.New(cfg, slog.New(slog.DiscardHandler), nodeid.Local{Value: "node-a"}, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.Members(); got.Len() != 0 {
		t.Fatalf("members = %#v", got.Values())
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledServiceRejectsInvalidSecret(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Gossip.Enabled = true
	cfg.Gossip.Bind = "127.0.0.1:0"
	cfg.Gossip.SecretKey = "too-short"
	svc := gossipsvc.New(cfg, slog.New(slog.DiscardHandler), nodeid.Local{Value: "node-a"}, nil)

	err := svc.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gossip.secret_key") {
		t.Fatalf("start error = %v", err)
	}
}

func TestEnabledServiceRejectsInvalidBind(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Gossip.Enabled = true
	cfg.Gossip.Bind = "missing-port"
	svc := gossipsvc.New(cfg, slog.New(slog.DiscardHandler), nodeid.Local{Value: "node-a"}, nil)

	err := svc.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gossip.bind") {
		t.Fatalf("start error = %v", err)
	}
}
