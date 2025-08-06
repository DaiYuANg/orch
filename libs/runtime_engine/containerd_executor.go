package runtime_engine

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

type ContainerdExecutor struct {
	client      *containerd.Client
	container   containerd.Container
	task        containerd.Task
	containerID string
	Image       string
}

// 启动容器
func (e *ContainerdExecutor) Start(ctx context.Context) error {
	if e.Image == "" {
		return fmt.Errorf("image is required")
	}

	// containerd 默认用 namespaces 管理资源，你可以根据你的集群需求改成别的
	ctx = namespaces.WithNamespace(ctx, "default")

	// 拉取镜像
	image, err := e.client.Pull(ctx, e.Image, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	containerID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	container, err := e.client.NewContainer(
		ctx,
		containerID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(containerID+"-snapshot", image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	e.container = container
	e.containerID = containerID

	// 创建task来运行容器
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	e.task = task

	// 启动容器
	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	return nil
}

// 停止容器
func (e *ContainerdExecutor) Stop(ctx context.Context) error {
	if e.task == nil {
		return fmt.Errorf("task not started")
	}

	exitStatusC, err := e.task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait error: %w", err)
	}

	// 发送停止信号
	if err := e.task.Kill(ctx, syscall.S_IFBLK); err != nil {
		return fmt.Errorf("failed to kill task: %w", err)
	}

	// 等待退出或者超时
	select {
	case <-exitStatusC:
	case <-time.After(10 * time.Second):
		// 强制kill
		_ = e.task.Kill(ctx, syscall.S_IFCHR)
	}

	// 删除任务和容器
	if _, err := e.task.Delete(ctx); err != nil {
		return err
	}
	if err := e.container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return err
	}

	e.task = nil
	e.container = nil
	e.containerID = ""

	return nil
}
