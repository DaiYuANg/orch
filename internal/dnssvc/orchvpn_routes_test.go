package dnssvc_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
)

func TestListWorkloadIPv4HostRoutes(t *testing.T) {
	t.Parallel()
	cfg := config.DNSConfig{
		Enabled: true,
		Listen:  "127.0.0.1:0",
	}
	cfg.Data.Path = filepath.Join(t.TempDir(), "dns.db")
	s := dnssvc.New(config.Config{DNS: cfg}, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Stop(ctx); err != nil {
			t.Fatalf("stop dns: %v", err)
		}
	})
	for workload, ip := range map[string]string{
		"a": "10.0.0.2",
		"b": "10.0.0.3",
		"c": "10.0.0.2",
	} {
		if err := s.UpsertWorkloadA(ctx, "default", workload, ip); err != nil {
			t.Fatalf("upsert %s: %v", workload, err)
		}
	}
	got := s.ListWorkloadIPv4HostRoutes()
	if len(got) != 2 || got[0] != "10.0.0.2/32" || got[1] != "10.0.0.3/32" {
		t.Fatalf("got %#v", got)
	}
}
