package engine

import "time"

type ResourceStatus string

const (
	StatusCreating ResourceStatus = "creating"
	StatusCreated  ResourceStatus = "created"
	StatusRunning  ResourceStatus = "running"
	StatusStopped  ResourceStatus = "stopped"
	StatusFailed   ResourceStatus = "failed"
	StatusErrored  ResourceStatus = "errored"
)

type ResourceID string

type ResourceInfo struct {
	ID        ResourceID
	Name      string
	Status    ResourceStatus
	CreatedAt time.Time
	Error     string
}

type ResourceDriver interface {
	Start(name string, options map[string]string) (ResourceID, error)
	Stop(id ResourceID) error
	GetStatus(id ResourceID) (ResourceInfo, error)
	List() ([]ResourceInfo, error)
}
