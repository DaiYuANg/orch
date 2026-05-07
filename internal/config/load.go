package config

import (
	"fmt"
	"strings"

	"github.com/arcgolabs/configx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// LoadFromCobra loads [Config] after Cobra has parsed flags. File from --config is merged before env; flag
// values override per [configx] priority. The --config flag is not applied as a config key.
func LoadFromCobra(cmd *cobra.Command) (Config, error) {
	cfgPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return Config{}, fmt.Errorf("cobra config flag: %w", err)
	}
	var opts []configx.Option
	if p := strings.TrimSpace(cfgPath); p != "" {
		opts = append(opts, configx.WithFiles(p))
	}
	opts = append(opts,
		configx.WithFlagSet(flagsForConfigMerge(cmd.Flags())),
		configx.WithArgsNameFunc(orchFlagToPath),
	)
	return Load(opts...)
}

// flagsForConfigMerge returns a FlagSet that shares the same *pflag.Flag values as fs but omits "config",
// so the config file path is not written into koanf as a leaf key.
func flagsForConfigMerge(fs *pflag.FlagSet) *pflag.FlagSet {
	out := pflag.NewFlagSet("orch-config-merge", pflag.ContinueOnError)
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "config" {
			return
		}
		out.AddFlag(f)
	})
	return out
}

// orchFlagToPath maps CLI flag names (before configx lowercasing) to dotted koanf paths.
// Defaults match configx defaultArgsName except where dotted paths must align with typed JSON keys.
func orchFlagToPath(name string) string {
	name = strings.TrimSpace(name)
	explicit := map[string]string{
		"scheduler-heartbeat-interval":              "scheduler.heartbeat_interval",
		"scheduler-resource-refresh-interval":       "scheduler.resource_refresh_interval",
		"scheduler-raft-leader-only":                "scheduler.raft_leader_only",
		"scheduler-max-concurrent-jobs":             "scheduler.max_concurrent_jobs",
		"scheduler-concurrent-jobs-mode":            "scheduler.concurrent_jobs_mode",
		"cluster-nodes":                             "cluster.nodes",
		"cluster-worker-token":                      "cluster.worker_token",
		"ingress-listen":                            "ingress.listen",
		"observability-prometheus-enabled":          "observability.prometheus.enabled",
		"observability-prometheus-path":             "observability.prometheus.path",
		"observability-prometheus-native-histogram": "observability.prometheus.native_histogram",
		"observability-otlp-enabled":                "observability.otlp.enabled",
		"observability-otlp-protocol":               "observability.otlp.protocol",
		"observability-otlp-endpoint":               "observability.otlp.endpoint",
		"observability-otlp-insecure":               "observability.otlp.insecure",
		"observability-otlp-service-name":           "observability.otlp.service_name",
		"dns-data-path":                             "dns.data.path",
		"orch-vpn-enabled":                          "orch_vpn.enabled",
		"orch-vpn-tunnel-listen-udp":                "orch_vpn.tunnel_listen_udp",
		"auth-jwt-secret":                           "auth.jwt.secret",
		"raft-node-id":                              "raft.node.id",
		"raft-badger-dir":                           "raft.badger.dir",
		"raft-bolt-path":                            "raft.bolt.path",
		"raft-snapshot-dir":                         "raft.snapshot.dir",
	}
	if p, ok := explicit[name]; ok {
		return p
	}
	return strings.ReplaceAll(strings.ToLower(name), "-", ".")
}
