package registry

import "time"

type RouteProtocol string

const (
	RouteProtocolHTTP RouteProtocol = "http"
	RouteProtocolTCP  RouteProtocol = "tcp"
	RouteProtocolUDP  RouteProtocol = "udp"
)

type ServiceEndpoint struct {
	ID        string            `json:"id"`
	Service   string            `json:"service"`
	NodeID    string            `json:"node_id"`
	NodeIP    string            `json:"node_ip"`
	Runtime   string            `json:"runtime"`
	Protocol  RouteProtocol     `json:"protocol"`
	Ports     map[string]int    `json:"ports,omitempty"`
	Healthy   bool              `json:"healthy"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Route struct {
	ID         string        `json:"id"`
	OwnerID    string        `json:"owner_id"`
	Service    string        `json:"service"`
	Protocol   RouteProtocol `json:"protocol"`
	Host       string        `json:"host,omitempty"`
	PathPrefix string        `json:"path_prefix,omitempty"`
	ListenPort int           `json:"listen_port,omitempty"`
	TargetPort int           `json:"target_port,omitempty"`
	PortName   string        `json:"port_name,omitempty"`
	Enabled    bool          `json:"enabled"`
	Source     string        `json:"source,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}
