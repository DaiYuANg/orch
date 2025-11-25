//go:build linux
// +build linux

package containerd

import (
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"golang.org/x/net/context"
)

func NewContainerdExecutor(socket string) (*ContainerdExecutor, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	return &ContainerdExecutor{
		client: client,
	}, nil
}

func (e *ContainerdExecutor) withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, "default")
}

// 拉取镜像
func (e *ContainerdExecutor) PullImage(ctx context.Context, image string) (containerd.Image, error) {
	ctx = e.withNamespace(ctx)
	return e.client.Pull(ctx, image, containerd.WithPullUnpack)
}

func (e *ContainerdExecutor) CreateContainer(ctx context.Context, id string, image containerd.Image, args []string) (containerd.Container, error) {
	ctx = e.withNamespace(ctx)

	return e.client.NewContainer(
		ctx,
		id,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(id+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithProcessArgs(args...),
		),
	)
}

func (e *ContainerdExecutor) StartContainer(ctx context.Context, container containerd.Container) (containerd.Task, <-chan containerd.ExitStatus, error) {
	ctx = e.withNamespace(ctx)

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return nil, nil, err
	}

	if err := task.Start(ctx); err != nil {
		return nil, nil, err
	}

	waitC, err := task.Wait(ctx)
	if err != nil {
		return nil, nil, err
	}

	return task, waitC, nil
}

func (e *ContainerdExecutor) RunContainer(ctx context.Context, id, image string, args []string) error {
	ctx = e.withNamespace(ctx)

	// 拉镜像
	img, err := e.PullImage(ctx, image)
	if err != nil {
		return fmt.Errorf("pull image failed: %w", err)
	}

	// 创建容器
	container, err := e.CreateContainer(ctx, id, img, args)
	if err != nil {
		return fmt.Errorf("create injector failed: %w", err)
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)

	// 启动容器
	task, statusC, err := e.StartContainer(ctx, container)
	if err != nil {
		return fmt.Errorf("start injector failed: %w", err)
	}
	defer task.Delete(ctx)

	// 等待退出
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return fmt.Errorf("get task result failed: %w", err)
	}

	fmt.Printf("injector %s exited with code %d\n", id, code)
	return nil
}
