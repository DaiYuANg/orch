package config

import (
	"strings"

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
		return []string{c.Addr}
	}
	return []string{":80", ":443"}
}

type DNSConfig struct {
	Enabled  bool
	Listen   string
	DataPath string
}

type SchedulerConfig struct {
	Enabled           bool
	HeartbeatInterval string
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
		configx.WithEnvPrefix("WARDEN"),
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
			Listen:  []string{":80", ":443"},
		},
		DNS: DNSConfig{
			Enabled:  true,
			Listen:   "127.0.0.1:15353",
			DataPath: "./data/dnsx.db",
		},
		Scheduler: SchedulerConfig{
			Enabled:           true,
			HeartbeatInterval: "2m",
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
