package config

import (
	"path/filepath"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/configx"
)

type Config struct {
	App           AppConfig           `json:"app"`
	Env           string              `json:"env"`
	Log           LogConfig           `json:"log"`
	HTTP          HTTPConfig          `json:"http"`
	Observability ObservabilityConfig `json:"observability"`
	Ingress       IngressConfig       `json:"ingress"`
	DNS           DNSConfig           `json:"dns"`
	Scheduler     SchedulerConfig     `json:"scheduler"`
	Auth          AuthConfig          `json:"auth"`
	Raft          RaftConfig          `json:"raft"`
}

type AppConfig struct {
	Name string `json:"name"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type HTTPConfig struct {
	Addr string `json:"addr"`
}

// ObservabilityConfig matches koanf paths like observability.prometheus.enabled (env ORCH_OBSERVABILITY_PROMETHEUS_ENABLED).
type ObservabilityConfig struct {
	Prometheus struct {
		Enabled         bool   `json:"enabled"`
		Path            string `json:"path"`
		NativeHistogram bool   `json:"native_histogram"` // classic + Prometheus native histogram on request duration (scrapers need native support)
	} `json:"prometheus"`
	// OTLP exports traces and metrics via OpenTelemetry (grpc default :4317, http default http://localhost:4318).
	OTLP struct {
		Enabled     bool   `json:"enabled"`
		Protocol    string `json:"protocol"` // grpc or http
		Endpoint    string `json:"endpoint"` // host:port for grpc; URL or host:port for http
		Insecure    bool   `json:"insecure"` // plaintext grpc; for http, use http:// or set insecure with host:port
		ServiceName string `json:"service_name"`
	} `json:"otlp"`
}

type IngressConfig struct {
	Enabled bool     `json:"enabled"`
	Addr    string   `json:"addr,omitempty"`
	Listen  []string `json:"listen,omitempty"`
}

// ListenAddrs returns ingress bind addresses: explicit Listen, else single Addr, else defaults ":80" and ":443".
func (c IngressConfig) ListenAddrs() []string {
	if len(c.Listen) > 0 {
		return c.Listen
	}
	if strings.TrimSpace(c.Addr) != "" {
		return list.NewList(c.Addr).Values()
	}
	return list.NewList(":80", ":443").Values()
}

// DNSConfig matches koanf paths like dns.data.path (env ORCH_DNS_DATA_PATH → dns.data.path).
type DNSConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Data    struct {
		Path string `json:"path"`
	} `json:"data"`
	Zone string `json:"zone,omitempty"`
}

type SchedulerConfig struct {
	HeartbeatInterval       string `json:"heartbeat_interval,omitempty"`
	ResourceRefreshInterval string `json:"resource_refresh_interval,omitempty"` // cadence for leader to apply local host metrics into Raft
	RaftLeaderOnly          bool   `json:"raft_leader_only,omitempty"`
	MaxConcurrentJobs       uint   `json:"max_concurrent_jobs,omitempty"`
	ConcurrentJobsMode      string `json:"concurrent_jobs_mode,omitempty"`
}

// AuthConfig matches paths auth.enabled and auth.jwt.secret.
type AuthConfig struct {
	Enabled bool `json:"enabled"`
	JWT     struct {
		Secret string `json:"secret"`
	} `json:"jwt"`
}

// RaftConfig matches raft.node.id, raft.badger.dir, raft.bolt.path, raft.snapshot.dir, etc.
type RaftConfig struct {
	Enabled bool `json:"enabled"`
	Node    struct {
		// ID forces this Raft/server identity (omit, "", or "auto" → resolved via nodeid hardware resolver).
		ID string `json:"id"`
	} `json:"node"`
	Bind   string `json:"bind"`
	Badger struct {
		Dir string `json:"dir"`
	} `json:"badger"`
	Bolt struct {
		Path string `json:"path"`
	} `json:"bolt"`
	Snapshot struct {
		Dir string `json:"dir"`
	} `json:"snapshot"`
}

// Load merges defaults, dotenv, optional files, env (ORCH_), then CLI flags when passed via [LoadFromCobra].
func Load(opts ...configx.Option) (Config, error) {
	base := []configx.Option{
		configx.WithTypedDefaults(Default()),
		configx.WithEnvPrefix("ORCH"),
		configx.WithPriority(
			configx.SourceDotenv,
			configx.SourceFile,
			configx.SourceEnv,
			configx.SourceArgs,
		),
		configx.WithValidateLevel(configx.ValidateLevelNone),
	}
	return configx.LoadTErr[Config](append(base, opts...)...)
}

func Default() Config {
	root := DefaultDataRoot()

	var dns DNSConfig
	dns.Enabled = true
	dns.Listen = "127.0.0.1:15353"
	dns.Data.Path = filepath.Join(root, "dnsx.db")
	dns.Zone = "orch.local"

	var obs ObservabilityConfig
	obs.Prometheus.Enabled = true
	obs.Prometheus.Path = "/metrics"
	obs.Prometheus.NativeHistogram = true

	var auth AuthConfig
	auth.Enabled = false
	auth.JWT.Secret = "dev-secret-change-me"

	var raft RaftConfig
	raft.Enabled = true
	// raft.Node.ID empty or "auto" → resolved from hardware at runtime ([nodeid.Resolve]).
	raft.Node.ID = ""
	raft.Bind = "127.0.0.1:7444"
	raft.Badger.Dir = filepath.Join(root, "raft-sched")
	raft.Bolt.Path = filepath.Join(root, "raft-meta.db")
	raft.Snapshot.Dir = filepath.Join(root, "raft-snapshots")

	return Config{
		App: AppConfig{
			Name: "orch",
		},
		Env: "dev",
		Log: LogConfig{
			Level: "debug",
		},
		HTTP: HTTPConfig{
			Addr: ":17443",
		},
		Observability: obs,
		Ingress: IngressConfig{
			Enabled: true,
			Listen:  list.NewList(":80", ":443").Values(),
		},
		DNS: dns,
		Scheduler: SchedulerConfig{
			HeartbeatInterval:       "2m",
			ResourceRefreshInterval: "30s",
			RaftLeaderOnly:          false,
			MaxConcurrentJobs:       0,
			ConcurrentJobsMode:      "reschedule",
		},
		Auth: auth,
		Raft: raft,
	}
}
