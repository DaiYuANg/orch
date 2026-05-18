package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arcgolabs/configx"
	"github.com/spf13/cobra"

	"github.com/lyonbrown4d/orch/internal/config"
)

func TestLoadFromCobraParsesClusterNodesFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "orch-server"}
	cmd.Flags().String("config", "", "config path")
	config.BindOrchFlags(cmd.Flags(), config.Default())
	if err := cmd.Flags().Parse([]string{"--cluster-nodes", "node-b=http://127.0.0.1:17446"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromCobra(cmd)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := cfg.Cluster.NodeURL("node-b")
	if !ok || got != "http://127.0.0.1:17446" {
		t.Fatalf("cluster node = %q, %v", got, ok)
	}
}

func TestLoadSupportsHCLConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orch.hcl")
	if err := os.WriteFile(path, []byte("env = \"hcl-test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configx.WithFiles(path))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "hcl-test" {
		t.Fatalf("env = %q", cfg.Env)
	}
}

func TestLoadFromCobraOverlaysChangedScalarFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "orch-server"}
	cmd.Flags().String("config", "", "config path")
	config.BindOrchFlags(cmd.Flags(), config.Default())
	if err := cmd.Flags().Parse([]string{
		"--http-addr", "127.0.0.1:17501",
		"--dns-enabled=false",
		"--ingress-enabled=false",
		"--observability-prometheus-enabled=false",
		"--raft-node-id", "node-a",
		"--raft-bind", "127.0.0.1:7451",
		"--raft-advertise", "127.0.0.1:7451",
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromCobra(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != "127.0.0.1:17501" {
		t.Fatalf("http addr = %q", cfg.HTTP.Addr)
	}
	if cfg.DNS.Enabled {
		t.Fatal("dns should be disabled")
	}
	if cfg.Ingress.Enabled {
		t.Fatal("ingress should be disabled")
	}
	if cfg.Observability.Prometheus.Enabled {
		t.Fatal("prometheus should be disabled")
	}
	if cfg.Raft.Node.ID != "node-a" {
		t.Fatalf("raft node id = %q", cfg.Raft.Node.ID)
	}
	if cfg.Raft.Bind != "127.0.0.1:7451" {
		t.Fatalf("raft bind = %q", cfg.Raft.Bind)
	}
	if cfg.Raft.Advertise != "127.0.0.1:7451" {
		t.Fatalf("raft advertise = %q", cfg.Raft.Advertise)
	}
}
