//go:build !linux

package systemd

import (
	"context"
	"strings"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Deploy(_ context.Context, _ deployv1.Metadata, _ deployv1.Workload) error {
	return oopsx.B("runtime", "systemd").Errorf("systemd runtime is only supported on linux")
}

func (p *Provider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return oopsx.B("runtime", "systemd").Errorf("systemd runtime is only supported on linux")
}

func (p *Provider) Status(_ context.Context, _ deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	return runtimeinfo.Status{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeSystemd,
		Status:  "unsupported",
		Message: "systemd runtime is only supported on linux",
	}, nil
}

func (p *Provider) Logs(_ context.Context, _ deployv1.Metadata, workloadName string, _ runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	return runtimeinfo.LogResult{}, oopsx.B("runtime", "systemd").Errorf("systemd logs are only supported on linux for workload %q", workloadName)
}
