package config

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadFromCobraParsesClusterNodesFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "orch-server"}
	cmd.Flags().String("config", "", "config path")
	BindOrchFlags(cmd.Flags(), Default())
	if err := cmd.Flags().Parse([]string{"--cluster-nodes", "node-b=http://127.0.0.1:17446"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromCobra(cmd)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := cfg.Cluster.NodeURL("node-b")
	if !ok || got != "http://127.0.0.1:17446" {
		t.Fatalf("cluster node = %q, %v", got, ok)
	}
}
