package task

import (
	"time"
)

type DeploymentStatus string

const (
	DeploymentStatusRunning DeploymentStatus = "running"
	DeploymentStatusFailed  DeploymentStatus = "failed"
	DeploymentStatusStopped DeploymentStatus = "stopped"
)

type InstanceStatus string

const (
	InstanceStatusRunning InstanceStatus = "running"
	InstanceStatusStopped InstanceStatus = "stopped"
	InstanceStatusFailed  InstanceStatus = "failed"
	InstanceStatusUnknown InstanceStatus = "unknown"
)

type DeployRequest struct {
	Filename string
	Format   string
	Content  string
}

type DeployResult struct {
	DeploymentID string `json:"deployment_id"`
	WorkloadName string `json:"workload_name"`
	Instances    int    `json:"instances"`
}

type DeploymentInfo struct {
	ID          string           `json:"id"`
	Workload    string           `json:"workload"`
	Format      string           `json:"format"`
	Status      DeploymentStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	InstanceIDs []string         `json:"instance_ids"`
	RouteIDs    []string         `json:"route_ids,omitempty"`
}

type InstanceInfo struct {
	ID                string         `json:"id"`
	DeploymentID      string         `json:"deployment_id"`
	Service           string         `json:"service"`
	Workload          string         `json:"workload"`
	Unit              string         `json:"unit"`
	Task              string         `json:"task"`
	Replica           int            `json:"replica"`
	Driver            string         `json:"driver"`
	ContainerID       string         `json:"container_id"`
	Status            InstanceStatus `json:"status"`
	LastError         string         `json:"last_error,omitempty"`
	RestartCount      int            `json:"restart_count"`
	ConsecutiveFailed int            `json:"consecutive_failed"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type DeploymentDetail struct {
	Deployment DeploymentInfo `json:"deployment"`
	Instances  []InstanceInfo `json:"instances"`
}

type deploymentRecord struct {
	DeploymentInfo
	instances map[string]*instanceRecord
}

type instanceRecord struct {
	InstanceInfo
	RunSpec     RuntimeRunSpec
	HealthCheck healthCheckSpec
	MaxRestarts int
	LastCheckAt time.Time
}

type healthCheckSpec struct {
	Type     string
	Path     string
	Command  string
	Port     int
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}
