//go:build !linux

package systemd

import (
	"context"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Deploy(_ context.Context, _ deployv1.Metadata, _ deployv1.Workload) error {
	return oopsx.B("runtime", "systemd").Errorf("systemd runtime is only supported on linux")
}

func (p *Provider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return oopsx.B("runtime", "systemd").Errorf("systemd runtime is only supported on linux")
}
