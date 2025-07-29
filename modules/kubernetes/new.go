package kubernetes

import "time"

type ResourceID string

type ResourceStatus string

const (
	StatusCreating ResourceStatus = "creating"
	StatusRunning  ResourceStatus = "running"
	StatusStopped  ResourceStatus = "stopped"
	StatusFailed   ResourceStatus = "failed"
)

type ResourceInfo struct {
	ID        ResourceID
	Name      string
	Status    ResourceStatus
	CreatedAt time.Time
	Error     string
}

// 通用资源操作接口（Firecracker、K8s都实现）
type ResourceDriver interface {
	Start(name string, options map[string]string) (ResourceID, error)
	Stop(id ResourceID) error
	GetStatus(id ResourceID) (ResourceInfo, error)
	List() ([]ResourceInfo, error)
}
