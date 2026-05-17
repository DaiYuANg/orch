//go:build linux

package containerd

import (
	"context"
	"errors"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/cenkalti/backoff/v5"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) prepareExistingCRIWorkload(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, meta deployv1.Metadata, w deployv1.Workload) (bool, error) {
	labels := criWorkloadLabelSelector(meta, w.Name)
	containers, err := runtime.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{LabelSelector: labels},
	})
	if err != nil {
		return false, oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list containers")
	}
	if ready, err := p.prepareExistingCRIContainers(ctx, runtime, meta, w, containers.GetContainers()); ready || err != nil {
		return ready, err
	}

	sandboxes, err := runtime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: &runtimeapi.PodSandboxFilter{LabelSelector: labels},
	})
	if err != nil {
		return false, oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list sandboxes")
	}
	if err := removeCRISandboxes(ctx, runtime, sandboxIDsFromList(sandboxes.GetItems())); err != nil {
		return false, err
	}
	return false, nil
}

func (p *Provider) prepareExistingCRIContainers(
	ctx context.Context,
	runtime runtimeapi.RuntimeServiceClient,
	meta deployv1.Metadata,
	w deployv1.Workload,
	containers []*runtimeapi.Container,
) (bool, error) {
	if len(containers) == 0 {
		return false, nil
	}

	running := false
	runningSandboxID := ""
	for _, ctr := range containers {
		if ctr == nil {
			continue
		}
		sandboxID := strings.TrimSpace(ctr.GetPodSandboxId())
		id := strings.TrimSpace(ctr.GetId())
		if id == "" {
			continue
		}
		status, err := criContainerStatus(ctx, runtime, id)
		if err != nil {
			return false, err
		}
		if status.GetState() == runtimeapi.ContainerState_CONTAINER_RUNNING {
			running = true
			if runningSandboxID == "" {
				runningSandboxID = sandboxID
			}
			continue
		}
		if err := removeCRIContainer(ctx, runtime, id); err != nil {
			return false, err
		}
	}
	if running {
		if err := p.recordCRIWorkloadDNS(ctx, runtime, meta, w, runningSandboxID); err != nil {
			return false, err
		}
		if p.logger != nil {
			p.logger.Info("containerd workload already running", "sandbox", runningSandboxID, "workload", w.Name)
		}
		return true, nil
	}
	return false, nil
}

func sandboxIDsFromList(items []*runtimeapi.PodSandbox) *set.Set[string] {
	out := set.NewSet[string]()
	list.NewList(items...).Range(func(_ int, sandbox *runtimeapi.PodSandbox) bool {
		if sandbox != nil && strings.TrimSpace(sandbox.GetId()) != "" {
			out.Add(sandbox.GetId())
		}
		return true
	})
	return out
}

func removeCRIContainer(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, containerID string) error {
	if strings.TrimSpace(containerID) == "" {
		return nil
	}
	_, _ = runtime.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: containerID, Timeout: criStopTimeout})
	if _, err := runtime.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containerID}); err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI remove container")
	}
	return nil
}

func removeCRISandboxes(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, sandboxIDs *set.Set[string]) error {
	var removeErr error
	sandboxIDs.Range(func(sandboxID string) bool {
		if strings.TrimSpace(sandboxID) == "" {
			return true
		}
		_, _ = runtime.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: sandboxID})
		if _, err := runtime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: sandboxID}); err != nil && removeErr == nil {
			removeErr = oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI remove sandbox")
		}
		return true
	})
	return removeErr
}

func (p *Provider) recordCRIWorkloadDNS(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, meta deployv1.Metadata, w deployv1.Workload, sandboxID string) error {
	if p.dns == nil {
		return nil
	}
	ip, err := waitSandboxIP(ctx, runtime, sandboxID)
	if err != nil {
		return err
	}
	return p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip)
}

func waitSandboxIP(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, sandboxID string) (string, error) {
	ip, err := backoff.Retry(ctx, func() (string, error) {
		status, err := runtime.PodSandboxStatus(ctx, &runtimeapi.PodSandboxStatusRequest{PodSandboxId: sandboxID})
		if err != nil {
			return "", err
		}
		if ip := strings.TrimSpace(status.GetStatus().GetNetwork().GetIp()); ip != "" {
			return ip, nil
		}
		return "", errSandboxIPPending
	}, backoff.WithBackOff(backoff.NewConstantBackOff(criSandboxIPDelay)), backoff.WithMaxTries(criSandboxIPAttempts))
	if err == nil {
		return ip, nil
	}
	if errors.Is(err, errSandboxIPPending) {
		return "", oopsx.B("runtime", "containerd").Errorf("timeout waiting for sandbox ip")
	}
	return "", oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI sandbox status")
}

func cleanupCRIWorkload(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, containerID, sandboxID string) {
	if strings.TrimSpace(containerID) != "" {
		_, _ = runtime.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: containerID, Timeout: criStopTimeout})
		_, _ = runtime.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containerID})
	}
	if strings.TrimSpace(sandboxID) != "" {
		_, _ = runtime.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: sandboxID})
		_, _ = runtime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: sandboxID})
	}
}
