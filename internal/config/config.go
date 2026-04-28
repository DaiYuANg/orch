package config

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/configx"
)

type Config struct {
	App           AppConfig
	Env           string
	Log           LogConfig
	HTTP          HTTPConfig
	Observability ObservabilityConfig
	Ingress       IngressConfig
	DNS           DNSConfig
	Scheduler     SchedulerConfig
	Auth          AuthConfig
	Raft          RaftConfig
}

type AppConfig struct {
	Name string
}

type LogConfig struct {
	Level string
}

type HTTPConfig struct {
	Addr string
}

type ObservabilityConfig struct {
	PrometheusEnabled bool
	PrometheusPath    string
}

type IngressConfig struct {
	Enabled bool
	// Addr is a single listener address (e.g. ":8088"). Ignored when Listen is non-empty.
	Addr string `yaml:"addr,omitempty"`
	// Listen binds embedded Caddy (HTTP app servers). Defaults to ":80" and ":443" when Addr and Listen are empty.
	Listen []string `yaml:"listen,omitempty"`
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

type DNSConfig struct {
	Enabled  bool
	Listen   string
	DataPath string
	// Zone is the authoritative DNS zone for orch service names (e.g. orch.local).
	Zone string `yaml:"zone,omitempty"`
}

type SchedulerConfig struct {
	Enabled           bool
	HeartbeatInterval string
	// RaftLeaderOnly wires gocron WithDistributedElector to Raft leadership:
	// only the Raft leader runs scheduled jobs when Raft is enabled.
	RaftLeaderOnly bool `yaml:"raftLeaderOnly,omitempty"`
	// MaxConcurrentJobs caps simultaneous jobs across the whole scheduler (gocron WithLimitConcurrentJobs).
	// 0 means unlimited.
	MaxConcurrentJobs uint `yaml:"maxConcurrentJobs,omitempty"`
	// ConcurrentJobsMode is reschedule or wait — maps to gocron LimitMode when MaxConcurrentJobs > 0.
	ConcurrentJobsMode string `yaml:"concurrentJobsMode,omitempty"`
}

type AuthConfig struct {
	Enabled   bool
	JWTSecret string
}

type RaftConfig struct {
	Enabled     bool
	NodeID      string
	Bind        string
	BadgerDir   string
	BoltPath    string
	SnapshotDir string
}

func Load() (Config, error) {
	return configx.LoadTErr[Config](
		configx.WithTypedDefaults(Default()),
		configx.WithEnvPrefix("ORCH"),
		configx.WithPriority(configx.SourceEnv),
		configx.WithValidateLevel(configx.ValidateLevelNone),
	)
}

func Default() Config {
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
		Observability: ObservabilityConfig{
			PrometheusEnabled: true,
			PrometheusPath:    "/metrics",
		},
		Ingress: IngressConfig{
			Enabled: true,
			Listen:  list.NewList(":80", ":443").Values(),
		},
		DNS: DNSConfig{
			Enabled:  true,
			Listen:   "127.0.0.1:15353",
			DataPath: "./data/dnsx.db",
			Zone:     "orch.local",
		},
		Scheduler: SchedulerConfig{
			Enabled:            true,
			HeartbeatInterval:  "2m",
			RaftLeaderOnly:     false,
			MaxConcurrentJobs:  0,
			ConcurrentJobsMode: "reschedule",
		},
		Auth: AuthConfig{
			Enabled:   false,
			JWTSecret: "dev-secret-change-me",
		},
		Raft: RaftConfig{
			Enabled:     true,
			NodeID:      "node-1",
			Bind:        "127.0.0.1:7444",
			BadgerDir:   "./data/raft-sched",
			BoltPath:    "./data/raft-meta.db",
			SnapshotDir: "./data/raft-snapshots",
		},
	}
}
