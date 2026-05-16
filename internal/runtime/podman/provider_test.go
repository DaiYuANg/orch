package podman_test

import (
	"log/slog"
	"testing"

	"github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	runtimedock "github.com/lyonbrown4d/orch/internal/runtime/podman"
)

func TestNewPodmanProviderKind(t *testing.T) {
	provider := runtimedock.NewProvider(slog.Default(), nil)
	if provider.Kind() != v1alpha1.RuntimePodman {
		t.Fatalf("kind = %q", provider.Kind())
	}
}
