//go:build linux

package containerd

import (
	"context"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	clients, err := dialCRI(ctx, containerdSocket())
	if err != nil {
		return err
	}
	defer clients.Close()

	labels := criWorkloadLabelSelector(meta, workloadName)
	containers, err := clients.runtime.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{LabelSelector: labels},
	})
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list containers")
	}

	sandboxIDs := set.NewSet[string]()
	containersList := list.NewList(containers.GetContainers()...)
	var stopErr error
	containersList.Range(func(_ int, ctr *runtimeapi.Container) bool {
		if ctr == nil {
			return true
		}
		if sandboxID := strings.TrimSpace(ctr.GetPodSandboxId()); sandboxID != "" {
			sandboxIDs.Add(sandboxID)
		}
		id := strings.TrimSpace(ctr.GetId())
		if id == "" {
			return true
		}
		if _, err := clients.runtime.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: id, Timeout: criStopTimeout}); err != nil && stopErr == nil {
			stopErr = oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI stop container")
		}
		if _, err := clients.runtime.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: id}); err != nil && stopErr == nil {
			stopErr = oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI remove container")
		}
		return true
	})
	if stopErr != nil {
		return stopErr
	}

	sandboxes, err := clients.runtime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: &runtimeapi.PodSandboxFilter{LabelSelector: labels},
	})
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list sandboxes")
	}
	list.NewList(sandboxes.GetItems()...).Range(func(_ int, sandbox *runtimeapi.PodSandbox) bool {
		if sandbox != nil && strings.TrimSpace(sandbox.GetId()) != "" {
			sandboxIDs.Add(sandbox.GetId())
		}
		return true
	})

	sandboxIDs.Range(func(sandboxID string) bool {
		if _, err := clients.runtime.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: sandboxID}); err != nil && stopErr == nil {
			stopErr = oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI stop sandbox")
		}
		if _, err := clients.runtime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: sandboxID}); err != nil && stopErr == nil {
			stopErr = oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI remove sandbox")
		}
		return true
	})
	if stopErr != nil {
		return stopErr
	}

	if p.dns != nil {
		if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
			return err
		}
	}
	p.logger.Info("containerd workload stopped", "workload", workloadName)
	return nil
}
