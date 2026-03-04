//go:build linux
// +build linux

package firecracker

type ResourceStatus string

const (
	StatusCreating ResourceStatus = "creating"
	StatusRunning  ResourceStatus = "running"
	StatusStopped  ResourceStatus = "stopped"
	StatusErrored  ResourceStatus = "errored"
)
