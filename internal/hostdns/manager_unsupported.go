//go:build !linux && !darwin && !windows

package hostdns

import (
	"context"
	"runtime"
)

type unsupportedManager struct{}

func DefaultManager() Manager {
	return &unsupportedManager{}
}

func (m *unsupportedManager) Install(context.Context, Config) error {
	return ErrUnsupported()
}

func (m *unsupportedManager) Uninstall(context.Context, Config) error {
	return nil
}

func (m *unsupportedManager) Status(_ context.Context, cfg Config) (Status, error) {
	return Status{Supported: false, Config: cfg, Detail: ErrUnsupported().Error()}, nil
}

func ErrUnsupported() error {
	return unsupportedError(runtime.GOOS)
}
