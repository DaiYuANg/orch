//go:build !windows
// +build !windows

package task

import "fmt"

func newWindowsServiceRuntimeExecutor() (RuntimeExecutor, error) {
	return nil, fmt.Errorf("windows-service runtime is only supported on windows")
}
