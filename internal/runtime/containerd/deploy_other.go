//go:build !linux

package containerd

import (
	"context"
	"fmt"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func (p *Provider) Deploy(_ context.Context, _ deployv1.Metadata, _ deployv1.Workload) error {
	return fmt.Errorf("containerd runtime is only supported on linux")
}

func (p *Provider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return fmt.Errorf("containerd runtime is only supported on linux")
}
