package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{logger: logger, dns: dns}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeDocker
}

func primaryIPv4(ns *types.NetworkSettings) string {
	if ns == nil {
		return ""
	}
	if ns.IPAddress != "" {
		return ns.IPAddress
	}
	for _, nw := range ns.Networks {
		if nw != nil && nw.IPAddress != "" {
			return nw.IPAddress
		}
	}
	return ""
}

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker: client: %w", err)
	}
	defer cli.Close()

	ref := workloadmeta.NormalizeImageRef(w.Run.Image)
	if ref == "" {
		return fmt.Errorf("docker: workload %q: run.image is required", w.Name)
	}

	pull, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("docker: pull %q: %w", ref, err)
	}
	_, _ = io.Copy(io.Discard, pull)
	_ = pull.Close()

	name := workloadmeta.OrchContainerName(meta, w.Name)
	cfg := &container.Config{
		Image:  ref,
		Labels: workloadmeta.Labels(meta, w),
	}

	hostCfg := &container.HostConfig{}
	if w.Run.Options.Docker != nil {
		if m := strings.TrimSpace(w.Run.Options.Docker.NetworkMode); m != "" {
			hostCfg.NetworkMode = container.NetworkMode(m)
		}
		hostCfg.Privileged = w.Run.Options.Docker.Privileged
	}

	createResp, err := cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		if errdefs.IsConflict(err) {
			return fmt.Errorf("docker: container %q already exists", name)
		}
		return fmt.Errorf("docker: create %q: %w", name, err)
	}

	if err := cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("docker: start %q: %w", createResp.ID, err)
	}

	inspect, err := cli.ContainerInspect(ctx, createResp.ID)
	if err != nil {
		_ = cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("docker: inspect after start: %w", err)
	}
	ip := primaryIPv4(inspect.NetworkSettings)
	if ip == "" {
		_ = cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("docker: no ipv4 address for container %s (ensure default bridge / or set networkMode)", name)
	}

	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		_ = cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return err
	}

	p.logger.Info("docker workload running", "container", name, "workload", w.Name, "ip", ip)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker: client: %w", err)
	}
	defer cli.Close()

	ns := workloadmeta.NamespaceOrDefault(meta.Namespace)
	fl := filters.NewArgs(
		filters.Arg("label", "orch.io/namespace="+ns),
		filters.Arg("label", "orch.io/workload="+strings.TrimSpace(workloadName)),
	)
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: fl})
	if err != nil {
		return fmt.Errorf("docker: list containers: %w", err)
	}
	if len(list) == 0 {
		_ = p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName)
		p.logger.Debug("docker stop: no container for workload", "workload", workloadName, "namespace", ns)
		return nil
	}
	id := list[0].ID
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("docker: remove container: %w", err)
	}
	if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
		return err
	}
	p.logger.Info("docker workload stopped", "workload", workloadName, "container", id)
	return nil
}
