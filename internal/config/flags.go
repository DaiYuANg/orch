package config

import (
	"github.com/spf13/pflag"
)

// BindOrchFlags registers CLI flags mapped by configx ([orchFlagToPath]) to dotted koanf paths.
func BindOrchFlags(fs *pflag.FlagSet, def Config) {
	fs.String("app-name", def.App.Name, "config path app.name")
	fs.String("env", def.Env, "config path env")
	fs.String("log-level", def.Log.Level, "config path log.level")
	fs.String("http-addr", def.HTTP.Addr, "config path http.addr")

	fs.Bool("observability-prometheus-enabled", def.Observability.Prometheus.Enabled, "config path observability.prometheus.enabled")
	fs.String("observability-prometheus-path", def.Observability.Prometheus.Path, "config path observability.prometheus.path")
	fs.Bool("observability-prometheus-native-histogram", def.Observability.Prometheus.NativeHistogram, "config path observability.prometheus.native_histogram")

	fs.Bool("observability-otlp-enabled", def.Observability.OTLP.Enabled, "config path observability.otlp.enabled")
	fs.String("observability-otlp-protocol", def.Observability.OTLP.Protocol, "config path observability.otlp.protocol (grpc or http)")
	fs.String("observability-otlp-endpoint", def.Observability.OTLP.Endpoint, "config path observability.otlp.endpoint")
	fs.Bool("observability-otlp-insecure", def.Observability.OTLP.Insecure, "config path observability.otlp.insecure")
	fs.String("observability-otlp-service-name", def.Observability.OTLP.ServiceName, "config path observability.otlp.service_name")

	fs.Bool("ingress-enabled", def.Ingress.Enabled, "config path ingress.enabled")
	fs.String("ingress-addr", def.Ingress.Addr, "config path ingress.addr")
	fs.StringSlice("ingress-listen", def.Ingress.Listen, "config path ingress.listen")

	fs.Bool("ingress-tls-enabled", def.Ingress.TLS.Enabled, "config path ingress.tls.enabled")
	fs.StringSlice("ingress-tls-listen", def.Ingress.TLS.Listen, "config path ingress.tls.listen (HTTPS binds, default :443)")
	fs.StringSlice("ingress-tls-domains", def.Ingress.TLS.Domains, "config path ingress.tls.domains (autocert host whitelist)")
	fs.String("ingress-tls-email", def.Ingress.TLS.Email, "config path ingress.tls.email (ACME account)")
	fs.String("ingress-tls-cache-dir", def.Ingress.TLS.CacheDir, "config path ingress.tls.cache_dir (PEM cache; default <data-dir>/autocert)")
	fs.Bool("ingress-tls-staging", def.Ingress.TLS.Staging, "config path ingress.tls.staging (Let's Encrypt staging)")

	fs.Bool("dns-enabled", def.DNS.Enabled, "config path dns.enabled")
	fs.String("dns-listen", def.DNS.Listen, "config path dns.listen")
	fs.String("dns-data-path", def.DNS.Data.Path, "config path dns.data.path")
	fs.String("dns-zone", def.DNS.Zone, "config path dns.zone")
	fs.String("dns-workload-nameserver", def.DNS.Workload.Nameserver, "config path dns.workload.nameserver (IP reachable by workloads on port 53)")
	fs.StringSlice("dns-workload-search", def.DNS.Workload.Search, "config path dns.workload.search")
	fs.StringSlice("dns-workload-upstream", def.DNS.Workload.Upstream, "config path dns.workload.upstream (upstream DNS servers for non-orch names)")
	fs.String("dns-workload-advertise-address", def.DNS.Workload.AdvertiseAddress, "config path dns.workload.advertise_address")

	fs.Bool("orch-vpn-enabled", def.OrchVPN.Enabled, "config path orch_vpn.enabled (UDP tunnel gateway)")
	fs.String("orch-vpn-tunnel-listen-udp", def.OrchVPN.TunnelListenUDP, "config path orch_vpn.tunnel_listen_udp")

	fs.String("scheduler-heartbeat-interval", def.Scheduler.HeartbeatInterval, "config path scheduler.heartbeat_interval")
	fs.String("scheduler-resource-refresh-interval", def.Scheduler.ResourceRefreshInterval, "config path scheduler.resource_refresh_interval")
	fs.Bool("scheduler-raft-leader-only", def.Scheduler.RaftLeaderOnly, "config path scheduler.raft_leader_only")
	fs.Uint("scheduler-max-concurrent-jobs", def.Scheduler.MaxConcurrentJobs, "config path scheduler.max_concurrent_jobs")
	fs.String("scheduler-concurrent-jobs-mode", def.Scheduler.ConcurrentJobsMode, "config path scheduler.concurrent_jobs_mode")

	fs.StringToString("cluster-nodes", def.Cluster.Nodes, "config path cluster.nodes (node_id=base_url, repeat or comma-separated)")
	fs.String("cluster-worker-token", def.Cluster.WorkerToken, "config path cluster.worker_token (bearer token for worker dispatch)")

	fs.Bool("auth-enabled", def.Auth.Enabled, "config path auth.enabled")
	fs.String("auth-jwt-secret", def.Auth.JWT.Secret, "config path auth.jwt.secret")

	fs.String("raft-node-id", def.Raft.Node.ID, "config path raft.node.id (empty or 'auto': OS host id / hardware fingerprint)")
	fs.String("raft-bind", def.Raft.Bind, "config path raft.bind")
	fs.String("raft-advertise", def.Raft.Advertise, "config path raft.advertise (host:port advertised to other raft peers)")
	fs.StringToString("raft-peers", def.Raft.Peers, "config path raft.peers (node_id=host:port, repeat or comma-separated static voters)")
	fs.Bool("raft-bootstrap", def.Raft.Bootstrap, "config path raft.bootstrap (bootstrap a new cluster when no raft state exists)")
	fs.String("raft-data-dir", def.Raft.Data.Dir, "config path raft.data.dir (Dragonboat NodeHost data)")
}
