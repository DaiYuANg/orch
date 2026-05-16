package podman

import (
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/client"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	runtimedocker "github.com/lyonbrown4d/orch/internal/runtime/docker"
)

type Provider struct {
	*runtimedocker.Provider
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		Provider: runtimedocker.NewProviderWithKind(
			logger,
			dns,
			deployv1.RuntimePodman,
			newPodmanClient,
		),
	}
}

func newPodmanClient() (*client.Client, error) {
	host := strings.TrimSpace(os.Getenv("PODMAN_HOST"))
	if host != "" {
		return client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	}
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}
