package docker

import (
	"context"
	"io"
	"log/slog"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
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

func primaryIPv4(ns *container.NetworkSettings) string {
	if ns == nil {
		return ""
	}
	for _, nw := range ns.Networks {
		if nw != nil && nw.IPAddress != "" {
			return nw.IPAddress
		}
	}
	return ""
}

func (p *Provider) drainDockerImagePull(ctx context.Context, cli *client.Client, ref string) error {
	pull, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return oopsx.B("runtime", "docker").Wrapf(err, "docker pull %q", ref)
	}
	defer func() {
		if closeErr := pull.Close(); closeErr != nil {
			p.logger.Warn("docker pull reader close", "error", closeErr)
		}
	}()
	if _, copyErr := io.Copy(io.Discard, pull); copyErr != nil {
		return oopsx.B("runtime", "docker").Wrapf(copyErr, "docker pull drain %q", ref)
	}
	return nil
}

func (p *Provider) deployWorkloadContainer(ctx context.Context, cli *client.Client, meta deployv1.Metadata, w deployv1.Workload, ref string) error {
	name := workloadmeta.OrchContainerName(meta, w.Name)
	ctrCfg := &container.Config{
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

	createResp, err := cli.ContainerCreate(ctx, ctrCfg, hostCfg, nil, nil, name)
	if err != nil {
		if cerrdefs.IsConflict(err) {
			return oopsx.B("runtime", "docker").Errorf("docker: container %q already exists", name)
		}
		return oopsx.B("runtime", "docker").Wrapf(err, "docker create %q", name)
	}

	return p.dockerRunAfterCreate(ctx, cli, meta, w, name, createResp.ID)
}

func (p *Provider) dockerRunAfterCreate(ctx context.Context, cli *client.Client, meta deployv1.Metadata, w deployv1.Workload, name, containerID string) error {
	removeFailed := func(stage string, cleanupErr error) {
		if rmErr := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); rmErr != nil {
			p.logger.Warn("docker: remove container after failure", "stage", stage, "remove_error", rmErr, "cause", cleanupErr)
		}
	}

	if startErr := cli.ContainerStart(ctx, containerID, container.StartOptions{}); startErr != nil {
		removeFailed("start", startErr)
		return oopsx.B("runtime", "docker").Wrapf(startErr, "docker start %q", containerID)
	}

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		removeFailed("inspect", err)
		return oopsx.B("runtime", "docker").Wrapf(err, "docker inspect after start")
	}
	ip := primaryIPv4(inspect.NetworkSettings)
	if ip == "" {
		removeFailed("no_ipv4", nil)
		return oopsx.B("runtime", "docker").Errorf("docker: no ipv4 address for container %s (ensure default bridge / or set networkMode)", name)
	}

	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		removeFailed("dns", err)
		return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
	}

	p.logger.Info("docker workload running", "container", name, "workload", w.Name, "ip", ip)
	return nil
}

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return oopsx.B("runtime", "docker").Wrapf(err, "docker client")
	}
	defer func() {
		if closeErr := cli.Close(); closeErr != nil {
			p.logger.Warn("docker client close", "error", closeErr)
		}
	}()

	ref := workloadmeta.NormalizeImageRef(w.Run.Image)
	if ref == "" {
		return oopsx.B("runtime", "docker").Errorf("docker: workload %q: run.image is required", w.Name)
	}

	if pullErr := p.drainDockerImagePull(ctx, cli, ref); pullErr != nil {
		return pullErr
	}
	return p.deployWorkloadContainer(ctx, cli, meta, w, ref)
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return oopsx.B("runtime", "docker").Wrapf(err, "docker client")
	}
	defer func() {
		if closeErr := cli.Close(); closeErr != nil {
			p.logger.Warn("docker client close", "error", closeErr)
		}
	}()

	ns := workloadmeta.NamespaceOrDefault(meta.Namespace)
	fl := filters.NewArgs(
		filters.Arg("label", "orch.io/namespace="+ns),
		filters.Arg("label", "orch.io/workload="+strings.TrimSpace(workloadName)),
	)
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: fl})
	if err != nil {
		return oopsx.B("runtime", "docker").Wrapf(err, "docker list containers")
	}
	if len(list) == 0 {
		if rmErr := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); rmErr != nil {
			p.logger.Warn("dns remove workload record", "error", rmErr)
		}
		p.logger.Debug("docker stop: no container for workload", "workload", workloadName, "namespace", ns)
		return nil
	}
	id := list[0].ID
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return oopsx.B("runtime", "docker").Wrapf(err, "docker remove container")
	}
	if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
		return oopsx.B("runtime", "dns").Wrapf(err, "remove workload DNS")
	}
	p.logger.Info("docker workload stopped", "workload", workloadName, "container", id)
	return nil
}
