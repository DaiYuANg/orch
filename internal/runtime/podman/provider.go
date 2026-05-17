package podman

import (
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/client"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	runtimedocker "github.com/lyonbrown4d/orch/internal/runtime/docker"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
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
		cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, oopsx.B("runtime", "podman").Wrapf(err, "podman client for host %q", host)
		}
		return cli, nil
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, oopsx.B("runtime", "podman").Wrapf(err, "podman client from env")
	}
	return cli, nil
}
