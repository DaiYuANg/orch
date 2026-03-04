//go:build !linux
// +build !linux

package task

import "fmt"

func newContainerdRuntimeExecutor() (RuntimeExecutor, error) {
	return nil, fmt.Errorf("containerd runtime is only supported on linux")
}
