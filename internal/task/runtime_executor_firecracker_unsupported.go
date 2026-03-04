//go:build !linux
// +build !linux

package task

import "fmt"

func newFirecrackerRuntimeExecutor() (RuntimeExecutor, error) {
	return nil, fmt.Errorf("firecracker runtime is only supported on linux")
}
