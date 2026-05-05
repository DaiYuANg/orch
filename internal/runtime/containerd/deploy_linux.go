//go:build linux

package containerd

import (
	"context"
	"fmt"
	"net"
	"os"
	goruntime "runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/vishvananda/netns"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const orchContainerdNamespace = "orch"

func containerdSocket() string {
	if v := strings.TrimSpace(os.Getenv("CONTAINERD_ADDRESS")); v != "" {
		return v
	}
	return "/run/containerd/containerd.sock"
}

func ipv4FromPID(pid int) (string, error) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	orig, err := netns.Get()
	if err != nil {
		return "", oopsx.B("runtime", "containerd").Wrapf(err, "netns current")
	}
	defer orig.Close()

	target, err := netns.GetFromPath(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return "", oopsx.B("runtime", "containerd").Wrapf(err, "open container netns (pid=%d)", pid)
	}
	defer target.Close()

	if err := netns.Set(target); err != nil {
		return "", oopsx.B("runtime", "containerd").Wrapf(err, "netns set container")
	}
	defer func() { _ = netns.Set(orig) }()

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.IP.String(), nil
		}
	}
	return "", oopsx.B("runtime", "containerd").Errorf("no usable ipv4 in container network namespace")
}

func waitIPv4(pid int, attempts int, delay time.Duration) (string, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		ip, err := ipv4FromPID(pid)
		if err == nil && ip != "" {
			return ip, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	if lastErr == nil {
		lastErr = oopsx.B("runtime", "containerd").New("timeout waiting for container ipv4")
	}
	return "", lastErr
}

func specOptsForWorkload(img containerd.Image, w deployv1.Workload) []oci.SpecOpts {
	opts := make([]oci.SpecOpts, 0, 6)
	if len(w.Run.Command) == 0 && len(w.Run.Args) > 0 {
		opts = append(opts, oci.WithImageConfigArgs(img, w.Run.Args))
	} else {
		opts = append(opts, oci.WithImageConfig(img))
	}
	if args := runconfig.CommandArgs(w.Run); len(args) > 0 {
		opts = append(opts, oci.WithProcessArgs(args...))
	}
	if env := runconfig.Env(w.Run.Env); len(env) > 0 {
		opts = append(opts, oci.WithEnv(env))
	}
	if cwd := strings.TrimSpace(w.Run.Cwd); cwd != "" {
		opts = append(opts, oci.WithProcessCwd(cwd))
	}
	if w.Resources != nil {
		if w.Resources.MemoryBytes > 0 {
			opts = append(opts, oci.WithMemoryLimit(uint64(w.Resources.MemoryBytes)))
		}
		if w.Resources.CPUMillis > 0 {
			quota, period := runconfig.CFSQuota(w.Resources.CPUMillis)
			opts = append(opts, oci.WithCPUCFS(quota, period))
		}
	}
	return opts
}

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	sock := containerdSocket()
	client, err := containerd.New(sock)
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd connect %s", sock)
	}
	defer client.Close()

	ctx = namespaces.Namespace(ctx, orchContainerdNamespace)

	ref := workloadmeta.NormalizeImageRef(w.Run.Image)
	if ref == "" {
		return oopsx.B("runtime", "containerd").Errorf("workload %q: run.image is required", w.Name)
	}

	img, err := client.Pull(ctx, ref, containerd.WithPullUnpack())
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd pull %q", ref)
	}

	cid := workloadmeta.OrchContainerName(meta, w.Name)
	snapKey := cid + "-snap"

	if _, err := client.LoadContainer(ctx, cid); err == nil {
		return oopsx.B("runtime", "containerd").Errorf("container %q already exists", cid)
	} else if err != nil && !errdefs.IsNotFound(err) {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd load container")
	}

	ctr, err := client.NewContainer(ctx, cid,
		containerd.WithContainerLabels(workloadmeta.Labels(meta, w)),
		containerd.WithNewSnapshot(snapKey, img),
		containerd.WithNewSpec(specOptsForWorkload(img, w)...),
	)
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd create %q", cid)
	}

	task, err := ctr.NewTask(ctx, cio.NullIO)
	if err != nil {
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd task %q", cid)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd start %q", cid)
	}

	pid := task.PID()
	if pid <= 0 {
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return oopsx.B("runtime", "containerd").Errorf("invalid task pid for %q", cid)
	}

	ip, err := waitIPv4(pid, 40, 50*time.Millisecond)
	if err != nil {
		_ = task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll)
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return oopsx.B("runtime", "containerd").Wrapf(err, "resolve ip for %q", cid)
	}

	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		_ = task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll)
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return err
	}

	p.logger.Info("containerd workload running", "container", cid, "workload", w.Name, "ip", ip)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	sock := containerdSocket()
	client, err := containerd.New(sock)
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd connect %s", sock)
	}
	defer client.Close()

	ctx = namespaces.Namespace(ctx, orchContainerdNamespace)

	nsWant := workloadmeta.NamespaceOrDefault(meta.Namespace)

	cs, err := client.Containers(ctx)
	if err != nil {
		return oopsx.B("runtime", "containerd").Wrapf(err, "containerd list")
	}

	for _, c := range cs {
		labels, err := c.Labels(ctx)
		if err != nil {
			continue
		}
		if labels["orch.io/workload"] != workloadName {
			continue
		}
		if workloadmeta.NamespaceOrDefault(labels["orch.io/namespace"]) != nsWant {
			continue
		}

		task, err := c.Task(ctx, nil)
		if err == nil && task != nil {
			_ = task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll)
			_, _ = task.Delete(ctx, containerd.WithProcessKill)
		}
		if err := c.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			return oopsx.B("runtime", "containerd").Wrapf(err, "containerd delete container")
		}
		if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
			return err
		}
		p.logger.Info("containerd workload stopped", "workload", workloadName)
		return nil
	}

	_ = p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName)
	return nil
}
