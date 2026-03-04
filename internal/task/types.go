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
	DesiredNode string           `json:"desired_node,omitempty"`
	WorkerNode  string           `json:"worker_node,omitempty"`
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
	NodeID            string         `json:"node_id,omitempty"`
	NodeIP            string         `json:"node_ip,omitempty"`
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

type InternalRunRequest struct {
	Driver string         `json:"driver,omitempty"`
	Spec   RuntimeRunSpec `json:"spec"`
}

type InternalRunResult struct {
	ContainerID string `json:"container_id"`
	Driver      string `json:"driver"`
	NodeID      string `json:"node_id"`
	NodeIP      string `json:"node_ip"`
}

type InternalStopRequest struct {
	ContainerID string `json:"container_id"`
	Driver      string `json:"driver,omitempty"`
}

type MigrateDeploymentRequest struct {
	TargetNode string `json:"target_node,omitempty"`
}

type MigrateDeploymentResult struct {
	DeploymentID string `json:"deployment_id"`
	Workload     string `json:"workload"`
	FromNode     string `json:"from_node"`
	ToNode       string `json:"to_node"`
	Instances    int    `json:"instances"`
	Migrated     int    `json:"migrated"`
}

type FailoverRequest struct {
	FailedNode string `json:"failed_node"`
	TargetNode string `json:"target_node,omitempty"`
}

type FailoverResult struct {
	FailedNode  string                    `json:"failed_node"`
	TargetNode  string                    `json:"target_node,omitempty"`
	Deployments int                       `json:"deployments"`
	Migrations  []MigrateDeploymentResult `json:"migrations"`
}

type RebalanceRequest struct {
	MaxMigrations int `json:"max_migrations,omitempty"`
}

type RebalanceResult struct {
	Workers    []string                  `json:"workers"`
	Candidates int                       `json:"candidates"`
	Applied    int                       `json:"applied"`
	Migrations []MigrateDeploymentResult `json:"migrations"`
}
