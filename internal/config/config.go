package config

import (
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
		Enabled bool   `json:"enabled"`
		Path    string `json:"path"`
	} `json:"prometheus"`
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
	Enabled            bool   `json:"enabled,omitempty"`
	HeartbeatInterval  string `json:"heartbeat_interval,omitempty"`
	RaftLeaderOnly     bool   `json:"raft_leader_only,omitempty"`
	MaxConcurrentJobs  uint   `json:"max_concurrent_jobs,omitempty"`
	ConcurrentJobsMode string `json:"concurrent_jobs_mode,omitempty"`
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
	var dns DNSConfig
	dns.Enabled = true
	dns.Listen = "127.0.0.1:15353"
	dns.Data.Path = "./data/dnsx.db"
	dns.Zone = "orch.local"

	var obs ObservabilityConfig
	obs.Prometheus.Enabled = true
	obs.Prometheus.Path = "/metrics"

	var auth AuthConfig
	auth.Enabled = false
	auth.JWT.Secret = "dev-secret-change-me"

	var raft RaftConfig
	raft.Enabled = true
	raft.Node.ID = "node-1"
	raft.Bind = "127.0.0.1:7444"
	raft.Badger.Dir = "./data/raft-sched"
	raft.Bolt.Path = "./data/raft-meta.db"
	raft.Snapshot.Dir = "./data/raft-snapshots"

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
			Enabled:            true,
			HeartbeatInterval:  "2m",
			RaftLeaderOnly:     false,
			MaxConcurrentJobs:  0,
			ConcurrentJobsMode: "reschedule",
		},
		Auth: auth,
		Raft: raft,
	}
}
