// Package config defines orch configuration schemas and loading helpers.
package config

type Config struct {
	App           AppConfig           `json:"app"`
	Env           string              `json:"env"`
	Log           LogConfig           `json:"log"`
	HTTP          HTTPConfig          `json:"http"`
	Observability ObservabilityConfig `json:"observability"`
	Ingress       IngressConfig       `json:"ingress"`
	DNS           DNSConfig           `json:"dns"`
	OrchVPN       OrchVPNConfig       `json:"orch_vpn"`
	Scheduler     SchedulerConfig     `json:"scheduler"`
	Cluster       ClusterConfig       `json:"cluster"`
	Gossip        GossipConfig        `json:"gossip"`
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
	// OTLP exports traces and metrics via OpenTelemetry (grpc default :4317, http default http://localhost:4318).
	OTLP struct {
		Enabled     bool   `json:"enabled"`
		Protocol    string `json:"protocol"` // grpc or http
		Endpoint    string `json:"endpoint"` // host:port for grpc; URL or host:port for http
		Insecure    bool   `json:"insecure"` // plaintext grpc; for http, use http:// or set insecure with host:port
		ServiceName string `json:"service_name"`
	} `json:"otlp"`
}
