//go:build !windows

package windowsservice

import (
	"context"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Deploy(_ context.Context, _ deployv1.Metadata, _ deployv1.Workload) error {
	return oopsx.B("runtime", "windows-service").Errorf("windows-service runtime is only supported on windows")
}

func (p *Provider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return oopsx.B("runtime", "windows-service").Errorf("windows-service runtime is only supported on windows")
}
