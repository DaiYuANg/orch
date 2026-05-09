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
	if err := overlayChangedFlags(&cfg, flags); err != nil {
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

func overlayChangedFlags(cfg *Config, fs *pflag.FlagSet) error {
	if cfg == nil || fs == nil {
		return nil
	}
	if flagChanged(fs, "app-name") {
		v, err := fs.GetString("app-name")
		if err != nil {
			return fmt.Errorf("cobra app-name flag: %w", err)
		}
		cfg.App.Name = v
	}
	if flagChanged(fs, "env") {
		v, err := fs.GetString("env")
		if err != nil {
			return fmt.Errorf("cobra env flag: %w", err)
		}
		cfg.Env = v
	}
	if flagChanged(fs, "log-level") {
		v, err := fs.GetString("log-level")
		if err != nil {
			return fmt.Errorf("cobra log-level flag: %w", err)
		}
		cfg.Log.Level = v
	}
	if flagChanged(fs, "http-addr") {
		v, err := fs.GetString("http-addr")
		if err != nil {
			return fmt.Errorf("cobra http-addr flag: %w", err)
		}
		cfg.HTTP.Addr = v
	}
	if flagChanged(fs, "observability-prometheus-enabled") {
		v, err := fs.GetBool("observability-prometheus-enabled")
		if err != nil {
			return fmt.Errorf("cobra observability-prometheus-enabled flag: %w", err)
		}
		cfg.Observability.Prometheus.Enabled = v
	}
	if flagChanged(fs, "observability-prometheus-path") {
		v, err := fs.GetString("observability-prometheus-path")
		if err != nil {
			return fmt.Errorf("cobra observability-prometheus-path flag: %w", err)
		}
		cfg.Observability.Prometheus.Path = v
	}
	if flagChanged(fs, "observability-prometheus-native-histogram") {
		v, err := fs.GetBool("observability-prometheus-native-histogram")
		if err != nil {
			return fmt.Errorf("cobra observability-prometheus-native-histogram flag: %w", err)
		}
		cfg.Observability.Prometheus.NativeHistogram = v
	}
	if flagChanged(fs, "observability-otlp-enabled") {
		v, err := fs.GetBool("observability-otlp-enabled")
		if err != nil {
			return fmt.Errorf("cobra observability-otlp-enabled flag: %w", err)
		}
		cfg.Observability.OTLP.Enabled = v
	}
	if flagChanged(fs, "observability-otlp-protocol") {
		v, err := fs.GetString("observability-otlp-protocol")
		if err != nil {
			return fmt.Errorf("cobra observability-otlp-protocol flag: %w", err)
		}
		cfg.Observability.OTLP.Protocol = v
	}
	if flagChanged(fs, "observability-otlp-endpoint") {
		v, err := fs.GetString("observability-otlp-endpoint")
		if err != nil {
			return fmt.Errorf("cobra observability-otlp-endpoint flag: %w", err)
		}
		cfg.Observability.OTLP.Endpoint = v
	}
	if flagChanged(fs, "observability-otlp-insecure") {
		v, err := fs.GetBool("observability-otlp-insecure")
		if err != nil {
			return fmt.Errorf("cobra observability-otlp-insecure flag: %w", err)
		}
		cfg.Observability.OTLP.Insecure = v
	}
	if flagChanged(fs, "observability-otlp-service-name") {
		v, err := fs.GetString("observability-otlp-service-name")
		if err != nil {
			return fmt.Errorf("cobra observability-otlp-service-name flag: %w", err)
		}
		cfg.Observability.OTLP.ServiceName = v
	}
	if flagChanged(fs, "ingress-enabled") {
		v, err := fs.GetBool("ingress-enabled")
		if err != nil {
			return fmt.Errorf("cobra ingress-enabled flag: %w", err)
		}
		cfg.Ingress.Enabled = v
	}
	if flagChanged(fs, "ingress-addr") {
		v, err := fs.GetString("ingress-addr")
		if err != nil {
			return fmt.Errorf("cobra ingress-addr flag: %w", err)
		}
		cfg.Ingress.Addr = v
	}
	if flagChanged(fs, "ingress-listen") {
		v, err := fs.GetStringSlice("ingress-listen")
		if err != nil {
			return fmt.Errorf("cobra ingress-listen flag: %w", err)
		}
		cfg.Ingress.Listen = v
	}
	if flagChanged(fs, "ingress-tls-enabled") {
		v, err := fs.GetBool("ingress-tls-enabled")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-enabled flag: %w", err)
		}
		cfg.Ingress.TLS.Enabled = v
	}
	if flagChanged(fs, "ingress-tls-listen") {
		v, err := fs.GetStringSlice("ingress-tls-listen")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-listen flag: %w", err)
		}
		cfg.Ingress.TLS.Listen = v
	}
	if flagChanged(fs, "ingress-tls-domains") {
		v, err := fs.GetStringSlice("ingress-tls-domains")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-domains flag: %w", err)
		}
		cfg.Ingress.TLS.Domains = v
	}
	if flagChanged(fs, "ingress-tls-email") {
		v, err := fs.GetString("ingress-tls-email")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-email flag: %w", err)
		}
		cfg.Ingress.TLS.Email = v
	}
	if flagChanged(fs, "ingress-tls-cache-dir") {
		v, err := fs.GetString("ingress-tls-cache-dir")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-cache-dir flag: %w", err)
		}
		cfg.Ingress.TLS.CacheDir = v
	}
	if flagChanged(fs, "ingress-tls-staging") {
		v, err := fs.GetBool("ingress-tls-staging")
		if err != nil {
			return fmt.Errorf("cobra ingress-tls-staging flag: %w", err)
		}
		cfg.Ingress.TLS.Staging = v
	}
	if flagChanged(fs, "dns-enabled") {
		v, err := fs.GetBool("dns-enabled")
		if err != nil {
			return fmt.Errorf("cobra dns-enabled flag: %w", err)
		}
		cfg.DNS.Enabled = v
	}
	if flagChanged(fs, "dns-listen") {
		v, err := fs.GetString("dns-listen")
		if err != nil {
			return fmt.Errorf("cobra dns-listen flag: %w", err)
		}
		cfg.DNS.Listen = v
	}
	if flagChanged(fs, "dns-data-path") {
		v, err := fs.GetString("dns-data-path")
		if err != nil {
			return fmt.Errorf("cobra dns-data-path flag: %w", err)
		}
		cfg.DNS.Data.Path = v
	}
	if flagChanged(fs, "dns-zone") {
		v, err := fs.GetString("dns-zone")
		if err != nil {
			return fmt.Errorf("cobra dns-zone flag: %w", err)
		}
		cfg.DNS.Zone = v
	}
	if flagChanged(fs, "dns-workload-nameserver") {
		v, err := fs.GetString("dns-workload-nameserver")
		if err != nil {
			return fmt.Errorf("cobra dns-workload-nameserver flag: %w", err)
		}
		cfg.DNS.Workload.Nameserver = v
	}
	if flagChanged(fs, "dns-workload-search") {
		v, err := fs.GetStringSlice("dns-workload-search")
		if err != nil {
			return fmt.Errorf("cobra dns-workload-search flag: %w", err)
		}
		cfg.DNS.Workload.Search = v
	}
	if flagChanged(fs, "dns-workload-upstream") {
		v, err := fs.GetStringSlice("dns-workload-upstream")
		if err != nil {
			return fmt.Errorf("cobra dns-workload-upstream flag: %w", err)
		}
		cfg.DNS.Workload.Upstream = v
	}
	if flagChanged(fs, "dns-workload-advertise-address") {
		v, err := fs.GetString("dns-workload-advertise-address")
		if err != nil {
			return fmt.Errorf("cobra dns-workload-advertise-address flag: %w", err)
		}
		cfg.DNS.Workload.AdvertiseAddress = v
	}
	if flagChanged(fs, "orch-vpn-enabled") {
		v, err := fs.GetBool("orch-vpn-enabled")
		if err != nil {
			return fmt.Errorf("cobra orch-vpn-enabled flag: %w", err)
		}
		cfg.OrchVPN.Enabled = v
	}
	if flagChanged(fs, "orch-vpn-tunnel-listen-udp") {
		v, err := fs.GetString("orch-vpn-tunnel-listen-udp")
		if err != nil {
			return fmt.Errorf("cobra orch-vpn-tunnel-listen-udp flag: %w", err)
		}
		cfg.OrchVPN.TunnelListenUDP = v
	}
	if flagChanged(fs, "scheduler-heartbeat-interval") {
		v, err := fs.GetString("scheduler-heartbeat-interval")
		if err != nil {
			return fmt.Errorf("cobra scheduler-heartbeat-interval flag: %w", err)
		}
		cfg.Scheduler.HeartbeatInterval = v
	}
	if flagChanged(fs, "scheduler-resource-refresh-interval") {
		v, err := fs.GetString("scheduler-resource-refresh-interval")
		if err != nil {
			return fmt.Errorf("cobra scheduler-resource-refresh-interval flag: %w", err)
		}
		cfg.Scheduler.ResourceRefreshInterval = v
	}
	if flagChanged(fs, "scheduler-raft-leader-only") {
		v, err := fs.GetBool("scheduler-raft-leader-only")
		if err != nil {
			return fmt.Errorf("cobra scheduler-raft-leader-only flag: %w", err)
		}
		cfg.Scheduler.RaftLeaderOnly = v
	}
	if flagChanged(fs, "scheduler-max-concurrent-jobs") {
		v, err := fs.GetUint("scheduler-max-concurrent-jobs")
		if err != nil {
			return fmt.Errorf("cobra scheduler-max-concurrent-jobs flag: %w", err)
		}
		cfg.Scheduler.MaxConcurrentJobs = v
	}
	if flagChanged(fs, "scheduler-concurrent-jobs-mode") {
		v, err := fs.GetString("scheduler-concurrent-jobs-mode")
		if err != nil {
			return fmt.Errorf("cobra scheduler-concurrent-jobs-mode flag: %w", err)
		}
		cfg.Scheduler.ConcurrentJobsMode = v
	}
	if fs.Changed("cluster-nodes") {
		nodes, err := fs.GetStringToString("cluster-nodes")
		if err != nil {
			return fmt.Errorf("cobra cluster-nodes flag: %w", err)
		}
		cfg.Cluster.Nodes = nodes
	}
	if flagChanged(fs, "cluster-worker-token") {
		v, err := fs.GetString("cluster-worker-token")
		if err != nil {
			return fmt.Errorf("cobra cluster-worker-token flag: %w", err)
		}
		cfg.Cluster.WorkerToken = v
	}
	if flagChanged(fs, "auth-enabled") {
		v, err := fs.GetBool("auth-enabled")
		if err != nil {
			return fmt.Errorf("cobra auth-enabled flag: %w", err)
		}
		cfg.Auth.Enabled = v
	}
	if flagChanged(fs, "auth-jwt-secret") {
		v, err := fs.GetString("auth-jwt-secret")
		if err != nil {
			return fmt.Errorf("cobra auth-jwt-secret flag: %w", err)
		}
		cfg.Auth.JWT.Secret = v
	}
	if flagChanged(fs, "raft-enabled") {
		v, err := fs.GetBool("raft-enabled")
		if err != nil {
			return fmt.Errorf("cobra raft-enabled flag: %w", err)
		}
		cfg.Raft.Enabled = v
	}
	if flagChanged(fs, "raft-node-id") {
		v, err := fs.GetString("raft-node-id")
		if err != nil {
			return fmt.Errorf("cobra raft-node-id flag: %w", err)
		}
		cfg.Raft.Node.ID = v
	}
	if flagChanged(fs, "raft-bind") {
		v, err := fs.GetString("raft-bind")
		if err != nil {
			return fmt.Errorf("cobra raft-bind flag: %w", err)
		}
		cfg.Raft.Bind = v
	}
	if flagChanged(fs, "raft-advertise") {
		v, err := fs.GetString("raft-advertise")
		if err != nil {
			return fmt.Errorf("cobra raft-advertise flag: %w", err)
		}
		cfg.Raft.Advertise = v
	}
	if fs.Changed("raft-peers") {
		peers, err := fs.GetStringToString("raft-peers")
		if err != nil {
			return fmt.Errorf("cobra raft-peers flag: %w", err)
		}
		cfg.Raft.Peers = peers
	}
	if flagChanged(fs, "raft-bootstrap") {
		v, err := fs.GetBool("raft-bootstrap")
		if err != nil {
			return fmt.Errorf("cobra raft-bootstrap flag: %w", err)
		}
		cfg.Raft.Bootstrap = v
	}
	if flagChanged(fs, "raft-data-dir") {
		v, err := fs.GetString("raft-data-dir")
		if err != nil {
			return fmt.Errorf("cobra raft-data-dir flag: %w", err)
		}
		cfg.Raft.Data.Dir = v
	}
	if flagChanged(fs, "raft-badger-dir") {
		v, err := fs.GetString("raft-badger-dir")
		if err != nil {
			return fmt.Errorf("cobra raft-badger-dir flag: %w", err)
		}
		cfg.Raft.Badger.Dir = v
	}
	if flagChanged(fs, "raft-bolt-path") {
		v, err := fs.GetString("raft-bolt-path")
		if err != nil {
			return fmt.Errorf("cobra raft-bolt-path flag: %w", err)
		}
		cfg.Raft.Bolt.Path = v
	}
	if flagChanged(fs, "raft-snapshot-dir") {
		v, err := fs.GetString("raft-snapshot-dir")
		if err != nil {
			return fmt.Errorf("cobra raft-snapshot-dir flag: %w", err)
		}
		cfg.Raft.Snapshot.Dir = v
	}
	return nil
}

func flagChanged(fs *pflag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	return f != nil && f.Changed
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
		"raft-data-dir":                             "raft.data.dir",
		"raft-badger-dir":                           "raft.badger.dir",
		"raft-bolt-path":                            "raft.bolt.path",
		"raft-snapshot-dir":                         "raft.snapshot.dir",
	}
	if p, ok := explicit[name]; ok {
		return p
	}
	return strings.ReplaceAll(strings.ToLower(name), "-", ".")
}
