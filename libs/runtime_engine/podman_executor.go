package runtime_engine

import (
	"github.com/containers/podman/v4/pkg/bindings"
	"golang.org/x/net/context"
)

type PodmanExecutor struct {
	conn        context.Context
	containerID string
	Image       string
	Args        []string
}

func NewPodmanExecutor(socketPath string) (*PodmanExecutor, error) {
	// 连接到Podman服务的socket，比如默认 /run/podman/podman.sock
	conn, err := bindings.NewConnection(context.Background(), "unix://"+socketPath)
	if err != nil {
		return nil, err
	}
	return &PodmanExecutor{conn: conn}, nil
}
