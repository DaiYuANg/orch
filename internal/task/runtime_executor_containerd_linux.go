//go:build linux
// +build linux

package task

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/errdefs"
	"github.com/google/uuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/samber/lo"
)

const defaultContainerdSocket = "/run/containerd/containerd.sock"

type containerdRuntimeExecutor struct {
	client    *containerd.Client
	socket    string
	namespace string
}

func newContainerdRuntimeExecutor() (RuntimeExecutor, error) {
	socket := strings.TrimSpace(os.Getenv("WARDEN_CONTAINERD_SOCKET"))
	if socket == "" {
		socket = defaultContainerdSocket
	}
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	return &containerdRuntimeExecutor{
		client:    client,
		socket:    socket,
		namespace: "default",
	}, nil
}

func (c *containerdRuntimeExecutor) Driver() string {
	return driverContainerd
}

func (c *containerdRuntimeExecutor) Ping(ctx context.Context) error {
	nsCtx := c.withNamespace(ctx)
	serving, err := c.client.IsServing(nsCtx)
	if err != nil {
		return err
	}
	if !serving {
		return fmt.Errorf("containerd is not serving on %s", c.socket)
	}
	return nil
}

func (c *containerdRuntimeExecutor) Run(ctx context.Context, spec RuntimeRunSpec) (string, error) {
	if strings.TrimSpace(spec.Image) == "" {
		return "", fmt.Errorf("containerd run requires image")
	}

	nsCtx := c.withNamespace(ctx)
	image, err := c.client.Pull(nsCtx, spec.Image, containerd.WithPullUnpack)
	if err != nil {
		return "", fmt.Errorf("containerd pull image %s: %w", spec.Image, err)
	}

	containerID := sanitizeContainerName(strings.TrimSpace(spec.Name))
	if containerID == "" || containerID == "warden-task" {
		containerID = "warden-" + uuid.NewString()
	}

	opts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot(containerID+"-snapshot", image),
		containerd.WithNewSpec(c.newContainerdSpecOptions(spec, image)...),
	}
	if len(spec.Labels) > 0 {
		opts = append(opts, containerd.WithContainerLabels(spec.Labels))
	}

	container, err := c.client.NewContainer(nsCtx, containerID, opts...)
	if err != nil {
		return "", fmt.Errorf("containerd create container %s: %w", containerID, err)
	}

	task, err := container.NewTask(nsCtx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		_ = container.Delete(nsCtx, containerd.WithSnapshotCleanup)
		return "", fmt.Errorf("containerd create task %s: %w", containerID, err)
	}

	if err := task.Start(nsCtx); err != nil {
		_, _ = task.Delete(nsCtx, containerd.WithProcessKill)
		_ = container.Delete(nsCtx, containerd.WithSnapshotCleanup)
		return "", fmt.Errorf("containerd start task %s: %w", containerID, err)
	}
	return containerID, nil
}

func (c *containerdRuntimeExecutor) Stop(ctx context.Context, containerID string) error {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return fmt.Errorf("container id is empty")
	}

	nsCtx := c.withNamespace(ctx)
	container, err := c.client.LoadContainer(nsCtx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	task, err := container.Task(nsCtx, nil)
	if err == nil {
		waitCh, waitErr := task.Wait(nsCtx)
		if waitErr == nil {
			_ = task.Kill(nsCtx, syscall.SIGTERM)
			select {
			case <-waitCh:
			case <-time.After(10 * time.Second):
				_ = task.Kill(nsCtx, syscall.SIGKILL)
				<-waitCh
			}
		}
		_, _ = task.Delete(nsCtx, containerd.WithProcessKill)
	}

	if err := container.Delete(nsCtx, containerd.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *containerdRuntimeExecutor) Status(ctx context.Context, containerID string) (RuntimeStatus, error) {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return RuntimeStatus{}, fmt.Errorf("container id is empty")
	}

	nsCtx := c.withNamespace(ctx)
	container, err := c.client.LoadContainer(nsCtx, id)
	if err != nil {
		return RuntimeStatus{}, err
	}
	task, err := container.Task(nsCtx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return RuntimeStatus{
				ContainerID: id,
				Name:        id,
				Running:     false,
				State:       "not_found",
			}, nil
		}
		return RuntimeStatus{}, err
	}
	status, err := task.Status(nsCtx)
	if err != nil {
		return RuntimeStatus{}, err
	}
	return RuntimeStatus{
		ContainerID: id,
		Name:        id,
		Running:     status.Status == containerd.Running,
		State:       string(status.Status),
		ExitCode:    int(status.ExitStatus),
	}, nil
}

func (c *containerdRuntimeExecutor) Logs(context.Context, string, int) (string, error) {
	return "", fmt.Errorf("containerd logs is not implemented yet")
}

func (c *containerdRuntimeExecutor) List(ctx context.Context, _ bool, filters map[string][]string) ([]RuntimeContainer, error) {
	nsCtx := c.withNamespace(ctx)
	containers, err := c.client.Containers(nsCtx)
	if err != nil {
		return nil, err
	}
	items := lo.FilterMap(containers, func(item containerd.Container, _ int) (RuntimeContainer, bool) {
		info, infoErr := item.Info(nsCtx)
		if infoErr != nil {
			return RuntimeContainer{}, false
		}
		if !matchContainerdLabels(info.Labels, filters) {
			return RuntimeContainer{}, false
		}
		return RuntimeContainer{
			ID:     item.ID(),
			Names:  []string{item.ID()},
			Image:  info.Image,
			Labels: info.Labels,
		}, true
	})
	return items, nil
}

func (c *containerdRuntimeExecutor) withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, c.namespace)
}

func (c *containerdRuntimeExecutor) newContainerdSpecOptions(spec RuntimeRunSpec, image containerd.Image) []oci.SpecOpts {
	options := []oci.SpecOpts{oci.WithImageConfig(image)}
	if len(spec.Cmd) > 0 {
		options = append(options, oci.WithProcessArgs(spec.Cmd...))
	}
	if envVars := mapToEnv(spec.Env); len(envVars) > 0 {
		options = append(options, oci.WithEnv(envVars))
	}
	if len(spec.Ports) > 0 {
		options = append(options, oci.WithHostNamespace(specs.NetworkNamespace))
	}
	return options
}

func matchContainerdLabels(labels map[string]string, filters map[string][]string) bool {
	labelFilters := filters["label"]
	if len(labelFilters) == 0 {
		return true
	}
	return lo.EveryBy(labelFilters, func(item string) bool {
		pair := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(pair) != 2 {
			return false
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		return labels[key] == value
	})
}
