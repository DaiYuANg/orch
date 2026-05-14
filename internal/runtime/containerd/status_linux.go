//go:build linux

package containerd

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Status(ctx context.Context, meta deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	clients, err := dialCRI(ctx, containerdSocket())
	if err != nil {
		return runtimeinfo.Status{}, err
	}
	defer clients.Close()

	out := stoppedContainerdStatus(workloadName)
	containerID, err := firstCRIContainerID(ctx, clients.runtime, meta, workloadName)
	if err != nil {
		return runtimeinfo.Status{}, err
	}
	if containerID == "" {
		return out, nil
	}
	status, err := criContainerStatus(ctx, clients.runtime, containerID)
	if err != nil {
		return runtimeinfo.Status{}, err
	}
	applyCRIContainerStatus(&out, status)
	return out, nil
}

func (p *Provider) Logs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	clients, err := dialCRI(ctx, containerdSocket())
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	defer clients.Close()

	containerID, err := firstCRIContainerID(ctx, clients.runtime, meta, workloadName)
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	if containerID == "" {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "containerd").Errorf("containerd container for workload %q not found", workloadName)
	}
	status, err := criContainerStatus(ctx, clients.runtime, containerID)
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	source := criContainerLogPath(p, meta, workloadName, status)
	content, err := runtimeinfo.ReadTailFile(source, opts.Tail)
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI read logs")
	}
	return runtimeinfo.LogResult{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeContainerd,
		Source:  source,
		Content: content,
	}, nil
}

func stoppedContainerdStatus(workloadName string) runtimeinfo.Status {
	return runtimeinfo.Status{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeContainerd,
		Status:  "stopped",
	}
}

func firstCRIContainerID(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, meta deployv1.Metadata, workloadName string) (string, error) {
	containers, err := runtime.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{LabelSelector: criWorkloadLabelSelector(meta, workloadName)},
	})
	if err != nil {
		return "", oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list containers")
	}
	for _, ctr := range containers.GetContainers() {
		if id := strings.TrimSpace(ctr.GetId()); id != "" {
			return id, nil
		}
	}
	return "", nil
}

func criContainerStatus(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, containerID string) (*runtimeapi.ContainerStatus, error) {
	resp, err := runtime.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{ContainerId: containerID, Verbose: true})
	if err != nil {
		return nil, oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI container status")
	}
	return resp.GetStatus(), nil
}

func applyCRIContainerStatus(out *runtimeinfo.Status, status *runtimeapi.ContainerStatus) {
	if status == nil {
		return
	}
	out.NativeID = strings.TrimSpace(status.GetId())
	out.Status = criContainerStateStatus(status.GetState())
	out.UpdatedAt = time.Now().UTC()
	out.Message = criContainerMessage(status)
	if startedAt := status.GetStartedAt(); startedAt > 0 {
		out.StartedAt = time.Unix(0, startedAt).UTC()
	}
}

func criContainerStateStatus(state runtimeapi.ContainerState) string {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return "created"
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return "running"
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return "exited"
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return "unknown"
	default:
		return strings.ToLower(strings.TrimPrefix(state.String(), "CONTAINER_"))
	}
}

func criContainerMessage(status *runtimeapi.ContainerStatus) string {
	if status == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if reason := strings.TrimSpace(status.GetReason()); reason != "" {
		parts = append(parts, reason)
	}
	if message := strings.TrimSpace(status.GetMessage()); message != "" && !slices.Contains(parts, message) {
		parts = append(parts, message)
	}
	if status.GetState() == runtimeapi.ContainerState_CONTAINER_EXITED {
		parts = append(parts, "exit code "+strconv.FormatInt(int64(status.GetExitCode()), 10))
	}
	return strings.Join(parts, ": ")
}

func criContainerLogPath(p *Provider, meta deployv1.Metadata, workloadName string, status *runtimeapi.ContainerStatus) string {
	logPath := strings.TrimSpace(status.GetLogPath())
	if logPath == "" {
		logPath = workloadmeta.SanitizeName(workloadName) + ".log"
	}
	if filepath.IsAbs(logPath) {
		return filepath.Clean(logPath)
	}
	return filepath.Join(p.logDir(meta, workloadName), logPath)
}
