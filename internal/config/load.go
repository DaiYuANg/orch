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
	flags := cobraConfigFlags(cmd)
	cfgPath, err := flags.GetString("config")
	if err != nil {
		return Config{}, fmt.Errorf("cobra config flag: %w", err)
	}
	var opts []configx.Option
	if p := strings.TrimSpace(cfgPath); p != "" {
		opts = append(opts, configx.WithFiles(p))
	}
	opts = append(opts,
		configx.WithFlagSet(flagsForConfigMerge(flags)),
		configx.WithArgsNameFunc(orchFlagToPath),
	)
	cfg, err := Load(opts...)
	if err != nil {
		return Config{}, err
	}
	if err := overlayStringMapFlags(&cfg, flags); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func cobraConfigFlags(cmd *cobra.Command) *pflag.FlagSet {
	out := pflag.NewFlagSet("orch-cobra-config", pflag.ContinueOnError)
	seen := map[string]struct{}{}
	add := func(fs *pflag.FlagSet) {
		if fs == nil {
			return
		}
		fs.VisitAll(func(f *pflag.Flag) {
			if _, ok := seen[f.Name]; ok {
				return
			}
			seen[f.Name] = struct{}{}
			out.AddFlag(f)
		})
	}
	add(cmd.InheritedFlags())
	add(cmd.Flags())
	return out
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

func overlayStringMapFlags(cfg *Config, fs *pflag.FlagSet) error {
	if cfg == nil || fs == nil {
		return nil
	}
	if fs.Changed("cluster-nodes") {
		nodes, err := fs.GetStringToString("cluster-nodes")
		if err != nil {
			return fmt.Errorf("cobra cluster-nodes flag: %w", err)
		}
		cfg.Cluster.Nodes = nodes
	}
	if fs.Changed("raft-peers") {
		peers, err := fs.GetStringToString("raft-peers")
		if err != nil {
			return fmt.Errorf("cobra raft-peers flag: %w", err)
		}
		cfg.Raft.Peers = peers
	}
	return nil
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
		"dns-workload-nameserver":                   "dns.workload.nameserver",
		"dns-workload-search":                       "dns.workload.search",
		"dns-workload-upstream":                     "dns.workload.upstream",
		"dns-workload-advertise-address":            "dns.workload.advertise_address",
		"orch-vpn-enabled":                          "orch_vpn.enabled",
		"orch-vpn-tunnel-listen-udp":                "orch_vpn.tunnel_listen_udp",
		"auth-jwt-secret":                           "auth.jwt.secret",
		"raft-node-id":                              "raft.node.id",
		"raft-advertise":                            "raft.advertise",
		"raft-peers":                                "raft.peers",
		"raft-bootstrap":                            "raft.bootstrap",
		"raft-badger-dir":                           "raft.badger.dir",
		"raft-bolt-path":                            "raft.bolt.path",
		"raft-snapshot-dir":                         "raft.snapshot.dir",
	}
	if p, ok := explicit[name]; ok {
		return p
	}
	return strings.ReplaceAll(strings.ToLower(name), "-", ".")
}
