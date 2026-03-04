//go:build !linux
// +build !linux

package task

import "fmt"

func newSystemdRuntimeExecutor() (RuntimeExecutor, error) {
	return nil, fmt.Errorf("systemd runtime is only supported on linux")
}
