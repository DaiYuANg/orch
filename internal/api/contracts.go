package api

import (
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/hostinfo"
	"github.com/daiyuang/orch/internal/services/registry"
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

// ListWorkloadsOutput is the response body envelope for GET PathV1Workloads.
type ListWorkloadsOutput struct {
	Body struct {
		Items []registry.WorkloadRecord `json:"items"`
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
