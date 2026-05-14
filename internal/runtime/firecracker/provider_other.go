//go:build !linux

package firecracker

import (
	"context"
	"strings"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Deploy(_ context.Context, _ deployv1.Metadata, _ deployv1.Workload) error {
	return oopsx.B("runtime", "firecracker").Errorf("firecracker runtime is only supported on linux")
}

func (p *Provider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return oopsx.B("runtime", "firecracker").Errorf("firecracker runtime is only supported on linux")
}

func (p *Provider) Status(_ context.Context, _ deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	return runtimeinfo.Status{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeFirecracker,
		Status:  "unsupported",
		Message: "firecracker runtime is only supported on linux",
	}, nil
}
