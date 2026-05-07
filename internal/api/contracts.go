package api

import (
	"time"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/hostinfo"
)

// EmptyInput is the request shape for handlers with no parameters or body.
type EmptyInput struct{}

// HealthOutput is the response body envelope for GET PathHealth.
type HealthOutput struct {
	Body struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	} `json:"body"`
}

// HostinfoOutput is the response body envelope for GET PathV1Hostinfo.
type HostinfoOutput struct {
	Body hostinfo.Report `json:"body"`
}

// WorkloadItem is the public API representation of a runtime registry record.
type WorkloadItem struct {
	Name      string    `json:"name"`
	Node      string    `json:"node,omitempty"`
	Runtime   string    `json:"runtime"`
	Artifact  string    `json:"artifact"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ListWorkloadsOutput is the response body envelope for GET PathV1Workloads.
type ListWorkloadsOutput struct {
	Body struct {
		Items *list.List[WorkloadItem] `json:"items"`
	} `json:"body"`
}

// AssignmentItem is the public API representation of a scheduler assignment.
type AssignmentItem struct {
	Key       string               `json:"key"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Node      string               `json:"node"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact"`
	Status    string               `json:"status"`
	Error     string               `json:"error,omitempty"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// ListAssignmentsOutput is the response body envelope for GET PathV1Assignments.
type ListAssignmentsOutput struct {
	Body struct {
		Items *list.List[AssignmentItem] `json:"items"`
	} `json:"body"`
}

// DeployInput is the request body envelope for POST PathV1Deploy.
type DeployInput struct {
	Body deployv1.App `json:"body"`
}

// DeploySourceInput is the request body envelope for POST PathV1DeploySource.
type DeploySourceInput struct {
	Body struct {
		// VirtualPath determines parsing (.orch uses plano; .yml/.yaml JSON-like YAML uses v1alpha1), e.g. "app.orch".
		VirtualPath string `json:"virtualPath"`
		Source      string `json:"source"`
	} `json:"body"`
}

// DeployOutput is the response body envelope for POST PathV1Deploy.
type DeployOutput struct {
	Body struct {
		Accepted  bool   `json:"accepted"`
		App       string `json:"app"`
		Workloads int    `json:"workloads"`
	} `json:"body"`
}

// OrchVPNBootstrapOutput is the response body for GET PathV1OrchVPNBootstrap.
type OrchVPNBootstrapOutput struct {
	Body struct {
		Enabled         bool               `json:"enabled"`
		APIVersion      string             `json:"api_version"`
		Encap           string             `json:"encap"`
		MTU             int                `json:"mtu"`
		TunnelUDPPort   int                `json:"tunnel_udp_port"`
		DNSZone         string             `json:"dns_zone"`
		ContainerRoutes *list.List[string] `json:"container_routes,omitempty"`
	} `json:"body"`
}
