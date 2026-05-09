package docker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/runtime/runtimeinfo"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
	"github.com/docker/docker/pkg/stdcopy"
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

func workloadContainerFilters(meta deployv1.Metadata, workloadName string) filters.Args {
	return filters.NewArgs(
		filters.Arg("label", "orch.io/app="+strings.TrimSpace(meta.Name)),
		filters.Arg("label", "orch.io/namespace="+workloadmeta.NamespaceOrDefault(meta.Namespace)),
		filters.Arg("label", "orch.io/workload="+strings.TrimSpace(workloadName)),
	)
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

func (p *Provider) ensureDockerImage(ctx context.Context, cli *client.Client, ref string) error {
	if err := p.drainDockerImagePull(ctx, cli, ref); err != nil {
		if _, inspectErr := cli.ImageInspect(ctx, ref); inspectErr == nil {
			p.logger.Warn("docker pull failed; using cached local image", "image", ref, "error", err)
			return nil
		}
		return err
	}
	return nil
}

func (p *Provider) prepareExistingDockerContainer(ctx context.Context, cli *client.Client, meta deployv1.Metadata, w deployv1.Workload, name string) (bool, error) {
	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, oopsx.B("runtime", "docker").Wrapf(err, "docker inspect existing container %q", name)
	}
	labels := map[string]string{}
	if inspect.Config != nil {
		labels = inspect.Config.Labels
	}
	if !workloadLabelsMatch(labels, meta, w) {
		return false, oopsx.B("runtime", "docker").Errorf("docker: container %q already exists and is not managed by app %s/%s workload %s",
			name, workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name, w.Name)
	}
	if inspect.State != nil && inspect.State.Running {
		if err := p.recordDockerWorkloadDNS(ctx, meta, w, name, inspect); err != nil {
			return false, err
		}
		p.logger.Info("docker workload already running", "container", name, "workload", w.Name)
		return true, nil
	}
	if err := cli.ContainerRemove(ctx, inspect.ID, container.RemoveOptions{Force: true}); err != nil {
		return false, oopsx.B("runtime", "docker").Wrapf(err, "docker remove stale container %q", name)
	}
	p.logger.Info("docker stale workload container removed", "container", name, "workload", w.Name)
	return false, nil
}

func (p *Provider) deployWorkloadContainer(ctx context.Context, cli *client.Client, meta deployv1.Metadata, w deployv1.Workload, ref string) error {
	name := workloadmeta.OrchContainerName(meta, w.Name)
	ctrCfg := &container.Config{
		Image:      ref,
		Entrypoint: w.Run.Exec.Command,
		Cmd:        w.Run.Exec.Args,
		Env:        runconfig.Env(w.EnvList()).Values(),
		WorkingDir: strings.TrimSpace(w.Run.Cwd),
		Labels:     containerLabels(meta, w),
	}

	hostCfg := &container.HostConfig{}
	if w.Resources != nil {
		hostCfg.Resources.Memory = w.Resources.MemoryBytes
		hostCfg.Resources.NanoCPUs = runconfig.NanoCPUs(w.Resources.CPUMillis)
	}
	if w.Run.Options.Docker != nil {
		if m := strings.TrimSpace(w.Run.Options.Docker.NetworkMode); m != "" {
			hostCfg.NetworkMode = container.NetworkMode(m)
		}
		hostCfg.Privileged = w.Run.Options.Docker.Privileged
	}
	applyWorkloadDNS(hostCfg, p.dns, meta.Namespace)

	createResp, err := cli.ContainerCreate(ctx, ctrCfg, hostCfg, nil, nil, name)
	if err != nil {
		if cerrdefs.IsConflict(err) {
			ready, prepErr := p.prepareExistingDockerContainer(ctx, cli, meta, w, name)
			if prepErr != nil {
				return prepErr
			}
			if ready {
				return nil
			}
			createResp, err = cli.ContainerCreate(ctx, ctrCfg, hostCfg, nil, nil, name)
			if err == nil {
				return p.dockerRunAfterCreate(ctx, cli, meta, w, name, createResp.ID)
			}
		}
		return oopsx.B("runtime", "docker").Wrapf(err, "docker create %q", name)
	}

	return p.dockerRunAfterCreate(ctx, cli, meta, w, name, createResp.ID)
}

func workloadLabelsMatch(labels map[string]string, meta deployv1.Metadata, w deployv1.Workload) bool {
	expected := workloadmeta.LabelMap(meta, w)
	matches := true
	expected.Range(func(key, want string) bool {
		if labels[key] != want {
			matches = false
			return false
		}
		return true
	})
	return matches
}

func containerLabels(meta deployv1.Metadata, w deployv1.Workload) map[string]string {
	labels := mapping.NewMapWithCapacity[string, string](4)
	if w.Run.Options.Docker != nil {
		w.Run.Options.Docker.LabelMap().Range(func(k, v string) bool {
			if key := strings.TrimSpace(k); key != "" {
				labels.Set(key, v)
			}
			return true
		})
	}
	workloadmeta.LabelMap(meta, w).Range(func(k, v string) bool {
		labels.Set(k, v)
		return true
	})
	return labels.All()
}

type workloadDNSResolver interface {
	WorkloadNameserver() (string, bool)
	WorkloadSearchDomains(namespace string) *list.List[string]
}

func applyWorkloadDNS(hostCfg *container.HostConfig, resolver workloadDNSResolver, namespace string) {
	if hostCfg == nil || resolver == nil {
		return
	}
	nameserver, ok := resolver.WorkloadNameserver()
	if !ok {
		return
	}
	hostCfg.DNS = []string{nameserver}
	if search := resolver.WorkloadSearchDomains(namespace); search.Len() > 0 {
		hostCfg.DNSSearch = search.Values()
	}
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
	if err := p.recordDockerWorkloadDNS(ctx, meta, w, name, inspect); err != nil {
		removeFailed("record_dns", err)
		return err
	}

	p.logger.Info("docker workload running", "container", name, "workload", w.Name)
	return nil
}

func (p *Provider) recordDockerWorkloadDNS(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload, name string, inspect container.InspectResponse) error {
	ip := primaryIPv4(inspect.NetworkSettings)
	if ip == "" {
		return oopsx.B("runtime", "docker").Errorf("docker: no ipv4 address for container %s (ensure default bridge / or set networkMode)", name)
	}
	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
	}
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

	ref := workloadmeta.NormalizeImageRef(w.Run.Artifact.Image)
	if ref == "" {
		return oopsx.B("runtime", "docker").Errorf("docker: workload %q: run.artifact.image is required", w.Name)
	}

	if pullErr := p.ensureDockerImage(ctx, cli, ref); pullErr != nil {
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
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: workloadContainerFilters(meta, workloadName)})
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

func (p *Provider) Status(ctx context.Context, meta deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", "docker").Wrapf(err, "docker client")
	}
	defer func() {
		if closeErr := cli.Close(); closeErr != nil {
			p.logger.Warn("docker client close", "error", closeErr)
		}
	}()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: workloadContainerFilters(meta, workloadName)})
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", "docker").Wrapf(err, "docker list containers")
	}
	out := runtimeinfo.Status{Name: strings.TrimSpace(workloadName), Runtime: deployv1.RuntimeDocker, Status: "stopped"}
	if len(containers) == 0 {
		return out, nil
	}
	inspect, err := cli.ContainerInspect(ctx, containers[0].ID)
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", "docker").Wrapf(err, "docker inspect container")
	}
	out.NativeID = inspect.ID
	if inspect.State != nil {
		out.Status = strings.TrimSpace(inspect.State.Status)
		if inspect.State.Running {
			out.Status = "running"
		}
		if startedAt := strings.TrimSpace(inspect.State.StartedAt); startedAt != "" {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, startedAt); parseErr == nil {
				out.StartedAt = parsed
			}
		}
		out.Message = strings.TrimSpace(inspect.State.Error)
	}
	out.UpdatedAt = time.Now().UTC()
	return out, nil
}

func (p *Provider) Logs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "docker").Wrapf(err, "docker client")
	}
	defer func() {
		if closeErr := cli.Close(); closeErr != nil {
			p.logger.Warn("docker client close", "error", closeErr)
		}
	}()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: workloadContainerFilters(meta, workloadName)})
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "docker").Wrapf(err, "docker list containers")
	}
	if len(containers) == 0 {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "docker").Errorf("docker container for workload %q not found", workloadName)
	}
	reader, err := cli.ContainerLogs(ctx, containers[0].ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(runtimeinfo.NormalizeTailLines(opts.Tail)),
	})
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "docker").Wrapf(err, "docker logs")
	}
	defer reader.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "docker").Wrapf(err, "docker logs decode")
	}
	content := stdout.String()
	if s := stderr.String(); s != "" {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += s
	}
	return runtimeinfo.LogResult{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeDocker,
		Source:  containers[0].ID,
		Content: content,
	}, nil
}
