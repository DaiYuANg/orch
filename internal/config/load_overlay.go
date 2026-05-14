package config

import (
	"fmt"

	"github.com/spf13/pflag"
)

func overlayCoreFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		stringFlag("app-name", func(v string) { cfg.App.Name = v }),
		stringFlag("env", func(v string) { cfg.Env = v }),
		stringFlag("log-level", func(v string) { cfg.Log.Level = v }),
		stringFlag("http-addr", func(v string) { cfg.HTTP.Addr = v }),
	)
}

func overlayObservabilityFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		boolFlag("observability-prometheus-enabled", func(v bool) { cfg.Observability.Prometheus.Enabled = v }),
		stringFlag("observability-prometheus-path", func(v string) { cfg.Observability.Prometheus.Path = v }),
		boolFlag("observability-otlp-enabled", func(v bool) { cfg.Observability.OTLP.Enabled = v }),
		stringFlag("observability-otlp-protocol", func(v string) { cfg.Observability.OTLP.Protocol = v }),
		stringFlag("observability-otlp-endpoint", func(v string) { cfg.Observability.OTLP.Endpoint = v }),
		boolFlag("observability-otlp-insecure", func(v bool) { cfg.Observability.OTLP.Insecure = v }),
		stringFlag("observability-otlp-service-name", func(v string) { cfg.Observability.OTLP.ServiceName = v }),
	)
}

func overlayIngressFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		boolFlag("ingress-enabled", func(v bool) { cfg.Ingress.Enabled = v }),
		stringFlag("ingress-addr", func(v string) { cfg.Ingress.Addr = v }),
		stringSliceFlag("ingress-listen", func(v []string) { cfg.Ingress.Listen = v }),
		boolFlag("ingress-tls-enabled", func(v bool) { cfg.Ingress.TLS.Enabled = v }),
		stringSliceFlag("ingress-tls-listen", func(v []string) { cfg.Ingress.TLS.Listen = v }),
		stringSliceFlag("ingress-tls-domains", func(v []string) { cfg.Ingress.TLS.Domains = v }),
		stringFlag("ingress-tls-email", func(v string) { cfg.Ingress.TLS.Email = v }),
		stringFlag("ingress-tls-cache-dir", func(v string) { cfg.Ingress.TLS.CacheDir = v }),
		boolFlag("ingress-tls-staging", func(v bool) { cfg.Ingress.TLS.Staging = v }),
	)
}

func overlayDNSFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		boolFlag("dns-enabled", func(v bool) { cfg.DNS.Enabled = v }),
		stringFlag("dns-listen", func(v string) { cfg.DNS.Listen = v }),
		stringFlag("dns-data-path", func(v string) { cfg.DNS.Data.Path = v }),
		stringFlag("dns-zone", func(v string) { cfg.DNS.Zone = v }),
		stringFlag("dns-workload-nameserver", func(v string) { cfg.DNS.Workload.Nameserver = v }),
		stringSliceFlag("dns-workload-search", func(v []string) { cfg.DNS.Workload.Search = v }),
		stringSliceFlag("dns-workload-upstream", func(v []string) { cfg.DNS.Workload.Upstream = v }),
		stringFlag("dns-workload-advertise-address", func(v string) { cfg.DNS.Workload.AdvertiseAddress = v }),
		boolFlag("orch-vpn-enabled", func(v bool) { cfg.OrchVPN.Enabled = v }),
		stringFlag("orch-vpn-tunnel-listen-udp", func(v string) { cfg.OrchVPN.TunnelListenUDP = v }),
	)
}

func overlaySchedulerFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		stringFlag("scheduler-heartbeat-interval", func(v string) { cfg.Scheduler.HeartbeatInterval = v }),
		stringFlag("scheduler-resource-refresh-interval", func(v string) { cfg.Scheduler.ResourceRefreshInterval = v }),
		boolFlag("scheduler-raft-leader-only", func(v bool) { cfg.Scheduler.RaftLeaderOnly = v }),
		uintFlag("scheduler-max-concurrent-jobs", func(v uint) { cfg.Scheduler.MaxConcurrentJobs = v }),
		stringFlag("scheduler-concurrent-jobs-mode", func(v string) { cfg.Scheduler.ConcurrentJobsMode = v }),
	)
}

func overlayClusterAuthFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		stringMapFlag("cluster-nodes", func(v map[string]string) { cfg.Cluster.Nodes = v }),
		stringFlag(clusterWorkerTokenFlag(), func(v string) { cfg.Cluster.WorkerToken = v }),
		boolFlag("auth-enabled", func(v bool) { cfg.Auth.Enabled = v }),
		stringFlag(authJWTSecretFlag(), func(v string) { cfg.Auth.JWT.Secret = v }),
	)
}

func overlayGossipFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		boolFlag("gossip-enabled", func(v bool) { cfg.Gossip.Enabled = v }),
		stringFlag("gossip-bind", func(v string) { cfg.Gossip.Bind = v }),
		stringFlag("gossip-advertise", func(v string) { cfg.Gossip.Advertise = v }),
		stringSliceFlag("gossip-seeds", func(v []string) { cfg.Gossip.Seeds = v }),
		stringFlag("gossip-secret-key", func(v string) { cfg.Gossip.SecretKey = v }),
		stringFlag("gossip-api-url", func(v string) { cfg.Gossip.APIURL = v }),
		boolFlag("gossip-auto-join-raft", func(v bool) { cfg.Gossip.AutoJoinRaft = v }),
		stringFlag("gossip-reconcile-interval", func(v string) { cfg.Gossip.ReconcileInterval = v }),
	)
}

func overlayRaftFlags(cfg *Config, fs *pflag.FlagSet) error {
	return applyFlagOverlays(fs,
		stringFlag("raft-node-id", func(v string) { cfg.Raft.Node.ID = v }),
		stringFlag("raft-bind", func(v string) { cfg.Raft.Bind = v }),
		stringFlag("raft-advertise", func(v string) { cfg.Raft.Advertise = v }),
		stringMapFlag("raft-peers", func(v map[string]string) { cfg.Raft.Peers = v }),
		boolFlag("raft-bootstrap", func(v bool) { cfg.Raft.Bootstrap = v }),
		stringFlag("raft-data-dir", func(v string) { cfg.Raft.Data.Dir = v }),
	)
}

type flagOverlay func(*pflag.FlagSet) error

func applyFlagOverlays(fs *pflag.FlagSet, overlays ...flagOverlay) error {
	for _, overlay := range overlays {
		if err := overlay(fs); err != nil {
			return err
		}
	}
	return nil
}

func stringFlag(name string, set func(string)) flagOverlay {
	return func(fs *pflag.FlagSet) error {
		if !flagChanged(fs, name) {
			return nil
		}
		v, err := fs.GetString(name)
		if err != nil {
			return fmt.Errorf("cobra %s flag: %w", name, err)
		}
		set(v)
		return nil
	}
}

func boolFlag(name string, set func(bool)) flagOverlay {
	return func(fs *pflag.FlagSet) error {
		if !flagChanged(fs, name) {
			return nil
		}
		v, err := fs.GetBool(name)
		if err != nil {
			return fmt.Errorf("cobra %s flag: %w", name, err)
		}
		set(v)
		return nil
	}
}

func uintFlag(name string, set func(uint)) flagOverlay {
	return func(fs *pflag.FlagSet) error {
		if !flagChanged(fs, name) {
			return nil
		}
		v, err := fs.GetUint(name)
		if err != nil {
			return fmt.Errorf("cobra %s flag: %w", name, err)
		}
		set(v)
		return nil
	}
}

func stringSliceFlag(name string, set func([]string)) flagOverlay {
	return func(fs *pflag.FlagSet) error {
		if !flagChanged(fs, name) {
			return nil
		}
		v, err := fs.GetStringSlice(name)
		if err != nil {
			return fmt.Errorf("cobra %s flag: %w", name, err)
		}
		set(v)
		return nil
	}
}

func stringMapFlag(name string, set func(map[string]string)) flagOverlay {
	return func(fs *pflag.FlagSet) error {
		if !fs.Changed(name) {
			return nil
		}
		v, err := fs.GetStringToString(name)
		if err != nil {
			return fmt.Errorf("cobra %s flag: %w", name, err)
		}
		set(v)
		return nil
	}
}
