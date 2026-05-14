//go:build linux

package containerd

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/cenkalti/backoff/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runconfig"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

const (
	criDialTimeout       = 5 * time.Second
	criSandboxIPAttempts = 40
	criSandboxIPDelay    = 50 * time.Millisecond
	criStopTimeout       = int64(10)
)

var errSandboxIPPending = errors.New("sandbox ip pending")

func containerdSocket() string {
	if v := strings.TrimSpace(os.Getenv("CONTAINERD_ADDRESS")); v != "" {
		return strings.TrimPrefix(strings.TrimSpace(v), "unix://")
	}
	return "/run/containerd/containerd.sock"
}

type criClients struct {
	conn    *grpc.ClientConn
	runtime runtimeapi.RuntimeServiceClient
	image   runtimeapi.ImageServiceClient
}

func dialCRI(ctx context.Context, socket string) (*criClients, error) {
	socket = strings.TrimPrefix(strings.TrimSpace(socket), "unix://")
	if socket == "" {
		socket = containerdSocket()
	}
	dialCtx, cancel := context.WithTimeout(ctx, criDialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		"unix://"+socket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI connect %s", socket)
	}
	return &criClients{
		conn:    conn,
		runtime: runtimeapi.NewRuntimeServiceClient(conn),
		image:   runtimeapi.NewImageServiceClient(conn),
	}, nil
}

func (c *criClients) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func criSandboxID(meta deployv1.Metadata, workloadName string) string {
	return workloadmeta.OrchContainerName(meta, workloadName)
}

func (p *Provider) logDir(meta deployv1.Metadata, workloadName string) string {
	return filepath.Join(p.rootOrDefault(), "logs", criSandboxID(meta, workloadName))
}

func (p *Provider) rootOrDefault() string {
	if strings.TrimSpace(p.root) != "" {
		return filepath.Clean(p.root)
	}
	return filepath.Join(config.DefaultDataRoot(), "runtime", "containerd")
}

func criWorkloadLabels(meta deployv1.Metadata, w deployv1.Workload) map[string]string {
	return workloadmeta.Labels(meta, w)
}

func criDNSConfig(dns workloadDNSResolver, namespace string) *runtimeapi.DNSConfig {
	if dns == nil {
		return nil
	}
	nameserver, ok := dns.WorkloadNameserver()
	if !ok {
		return nil
	}
	cfg := &runtimeapi.DNSConfig{Servers: []string{nameserver}}
	if search := dns.WorkloadSearchDomains(namespace); search.Len() > 0 {
		cfg.Searches = search.Values()
	}
	return cfg
}

type workloadDNSResolver interface {
	WorkloadNameserver() (string, bool)
	WorkloadSearchDomains(namespace string) *list.List[string]
}

func criSandboxConfig(p *Provider, meta deployv1.Metadata, w deployv1.Workload) *runtimeapi.PodSandboxConfig {
	ns := workloadmeta.NamespaceOrDefault(meta.Namespace)
	name := criSandboxID(meta, w.Name)
	return &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      name,
			Uid:       name,
			Namespace: ns,
			Attempt:   0,
		},
		Hostname:     workloadmeta.SanitizeName(w.Name),
		LogDirectory: p.logDir(meta, w.Name),
		DnsConfig:    criDNSConfig(p.dns, ns),
		Labels:       criWorkloadLabels(meta, w),
		Annotations: map[string]string{
			"orch.io/app":       meta.Name,
			"orch.io/namespace": ns,
			"orch.io/workload":  w.Name,
		},
		Linux: &runtimeapi.LinuxPodSandboxConfig{},
	}
}

func criEnv(vars *list.List[deployv1.EnvVar]) []*runtimeapi.KeyValue {
	out := list.NewListWithCapacity[*runtimeapi.KeyValue](vars.Len())
	vars.Range(func(_ int, v deployv1.EnvVar) bool {
		name := strings.TrimSpace(v.Name)
		if name == "" {
			return true
		}
		out.Add(&runtimeapi.KeyValue{Key: name, Value: v.Value})
		return true
	})
	return out.Values()
}

func criLinuxResources(w deployv1.Workload) *runtimeapi.LinuxContainerResources {
	if w.Resources == nil {
		return nil
	}
	res := &runtimeapi.LinuxContainerResources{}
	if w.Resources.MemoryBytes > 0 {
		res.MemoryLimitInBytes = w.Resources.MemoryBytes
	}
	if w.Resources.CPUMillis > 0 {
		quota, period := runconfig.CFSQuota(w.Resources.CPUMillis)
		res.CpuQuota = quota
		res.CpuPeriod = int64(period)
	}
	if res.MemoryLimitInBytes == 0 && res.CpuQuota == 0 && res.CpuPeriod == 0 {
		return nil
	}
	return res
}

func criContainerConfig(ref string, meta deployv1.Metadata, w deployv1.Workload) *runtimeapi.ContainerConfig {
	cfg := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name:    workloadmeta.SanitizeName(w.Name),
			Attempt: 0,
		},
		Image:      &runtimeapi.ImageSpec{Image: ref},
		Command:    w.Run.Exec.Command,
		Args:       w.Run.Exec.Args,
		WorkingDir: strings.TrimSpace(w.Run.Cwd),
		Envs:       criEnv(w.EnvList()),
		Labels:     criWorkloadLabels(meta, w),
		LogPath:    workloadmeta.SanitizeName(w.Name) + ".log",
	}
	if res := criLinuxResources(w); res != nil {
		cfg.Linux = &runtimeapi.LinuxContainerConfig{Resources: res}
	}
	return cfg
}

func ensureNoExistingWorkload(ctx context.Context, runtime runtimeapi.RuntimeServiceClient, meta deployv1.Metadata, workloadName string) error {
	labels := map[string]string{
		"orch.io/namespace": workloadmeta.NamespaceOrDefault(meta.Namespace),
		"orch.io/workload":  strings.TrimSpace(workloadName),
	}
	containers, err := runtime.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{LabelSelector: labels},
	})
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list containers")
	}
	if len(containers.GetContainers()) > 0 {
		return oopsx.B("runtime", "containerd").Errorf("containerd workload %q already exists", workloadName)
	}
	sandboxes, err := runtime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: &runtimeapi.PodSandboxFilter{LabelSelector: labels},
	})
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI list sandboxes")
	}
	if len(sandboxes.GetItems()) > 0 {
		return oopsx.B("runtime", "containerd").Errorf("containerd sandbox for workload %q already exists", workloadName)
	}
	return nil
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

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	clients, err := dialCRI(ctx, containerdSocket())
	if err != nil {
		return err
	}
	defer clients.Close()

	ref := workloadmeta.NormalizeImageRef(w.Run.Artifact.Image)
	if ref == "" {
		return oopsx.B("runtime", "containerd").Errorf("workload %q: run.artifact.image is required", w.Name)
	}
	if err := ensureNoExistingWorkload(ctx, clients.runtime, meta, w.Name); err != nil {
		return err
	}

	sandboxCfg := criSandboxConfig(p, meta, w)
	if err := os.MkdirAll(sandboxCfg.LogDirectory, 0o755); err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "create containerd CRI log dir")
	}

	if _, err := clients.image.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image:         &runtimeapi.ImageSpec{Image: ref},
		SandboxConfig: sandboxCfg,
	}); err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI pull %q", ref)
	}

	sandbox, err := clients.runtime.RunPodSandbox(ctx, &runtimeapi.RunPodSandboxRequest{Config: sandboxCfg})
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI run sandbox")
	}
	sandboxID := sandbox.GetPodSandboxId()

	containerCfg := criContainerConfig(ref, meta, w)
	created, err := clients.runtime.CreateContainer(ctx, &runtimeapi.CreateContainerRequest{
		PodSandboxId:  sandboxID,
		Config:        containerCfg,
		SandboxConfig: sandboxCfg,
	})
	if err != nil {
		cleanupCRIWorkload(ctx, clients.runtime, "", sandboxID)
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI create container")
	}
	containerID := created.GetContainerId()

	if _, err := clients.runtime.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: containerID}); err != nil {
		cleanupCRIWorkload(ctx, clients.runtime, containerID, sandboxID)
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd CRI start container")
	}

	ip, err := waitSandboxIP(ctx, clients.runtime, sandboxID)
	if err != nil {
		cleanupCRIWorkload(ctx, clients.runtime, containerID, sandboxID)
		return err
	}

	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		cleanupCRIWorkload(ctx, clients.runtime, containerID, sandboxID)
		return err
	}

	p.logger.Info("containerd workload running", "sandbox", sandboxID, "container", containerID, "workload", w.Name, "ip", ip)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	clients, err := dialCRI(ctx, containerdSocket())
	if err != nil {
		return err
	}
	defer clients.Close()

	labels := map[string]string{
		"orch.io/namespace": workloadmeta.NamespaceOrDefault(meta.Namespace),
		"orch.io/workload":  strings.TrimSpace(workloadName),
	}
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

	if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
		return err
	}
	p.logger.Info("containerd workload stopped", "workload", workloadName)
	return nil
}
