//go:build linux
// +build linux

package task

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	firecrackerrt "github.com/DaiYuANg/warden/internal/runtime_engine/firecracker"
	"github.com/samber/lo"
)

const defaultFirecrackerBasePath = "/tmp/warden-firecracker"

type firecrackerRuntimeExecutor struct {
	manager *firecrackerrt.FirecrackerManager

	mu       sync.RWMutex
	labelsBy map[string]map[string]string
	imageBy  map[string]string
}

func newFirecrackerRuntimeExecutor() (RuntimeExecutor, error) {
	basePath := strings.TrimSpace(os.Getenv("WARDEN_FIRECRACKER_BASE_PATH"))
	if basePath == "" {
		basePath = defaultFirecrackerBasePath
	}
	return &firecrackerRuntimeExecutor{
		manager:  firecrackerrt.NewManager(basePath),
		labelsBy: make(map[string]map[string]string),
		imageBy:  make(map[string]string),
	}, nil
}

func (f *firecrackerRuntimeExecutor) Driver() string {
	return driverFirecracker
}

func (f *firecrackerRuntimeExecutor) Ping(context.Context) error {
	if _, err := exec.LookPath("firecracker"); err != nil {
		return fmt.Errorf("firecracker binary not found: %w", err)
	}
	return nil
}

func (f *firecrackerRuntimeExecutor) Run(ctx context.Context, spec RuntimeRunSpec) (string, error) {
	rootfs := strings.TrimSpace(spec.Image)
	if rootfs == "" {
		return "", fmt.Errorf("firecracker run requires rootfs path in image field")
	}
	kernelPath := strings.TrimSpace(lo.Ternary(spec.Env != nil, spec.Env["WARDEN_FIRECRACKER_KERNEL"], ""))
	if kernelPath == "" {
		kernelPath = strings.TrimSpace(os.Getenv("WARDEN_FIRECRACKER_KERNEL"))
	}
	if kernelPath == "" {
		return "", fmt.Errorf("firecracker kernel path is required via env WARDEN_FIRECRACKER_KERNEL")
	}

	vcpus := int64(max(1, mustAtoi(lo.Ternary(spec.Env != nil, spec.Env["WARDEN_FIRECRACKER_VCPUS"], ""), 1)))
	memMiB := int64(max(128, mustAtoi(lo.Ternary(spec.Env != nil, spec.Env["WARDEN_FIRECRACKER_MEM_MIB"], ""), 128)))

	vmID, err := f.manager.StartVM(ctx, firecrackerrt.VMOptions{
		KernelImagePath: kernelPath,
		RootFSPath:      rootfs,
		VCPUs:           vcpus,
		MemMiB:          memMiB,
	})
	if err != nil {
		return "", err
	}

	id := string(vmID)
	f.mu.Lock()
	f.labelsBy[id] = cloneStringMap(spec.Labels)
	f.imageBy[id] = rootfs
	f.mu.Unlock()
	return id, nil
}

func (f *firecrackerRuntimeExecutor) Stop(ctx context.Context, containerID string) error {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return fmt.Errorf("container id is empty")
	}
	if err := f.manager.StopVM(ctx, firecrackerrt.VMID(id)); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		return err
	}

	f.mu.Lock()
	delete(f.labelsBy, id)
	delete(f.imageBy, id)
	f.mu.Unlock()
	return nil
}

func (f *firecrackerRuntimeExecutor) Status(_ context.Context, containerID string) (RuntimeStatus, error) {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return RuntimeStatus{}, fmt.Errorf("container id is empty")
	}
	status, err := f.manager.GetStatus(firecrackerrt.VMID(id))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return RuntimeStatus{
				ContainerID: id,
				Name:        id,
				Running:     false,
				State:       "not_found",
			}, nil
		}
		return RuntimeStatus{}, err
	}
	return RuntimeStatus{
		ContainerID: id,
		Name:        id,
		Running:     status == firecrackerrt.StatusRunning,
		State:       string(status),
	}, nil
}

func (f *firecrackerRuntimeExecutor) Logs(context.Context, string, int) (string, error) {
	return "", fmt.Errorf("firecracker logs is not implemented yet")
}

func (f *firecrackerRuntimeExecutor) List(_ context.Context, _ bool, filters map[string][]string) ([]RuntimeContainer, error) {
	vmByID := lo.SliceToMap(f.manager.ListVMs(), func(item firecrackerrt.VMInstance) (string, firecrackerrt.VMInstance) {
		return string(item.ID), item
	})

	f.mu.RLock()
	defer f.mu.RUnlock()
	return lo.FilterMap(lo.Keys(f.labelsBy), func(id string, _ int) (RuntimeContainer, bool) {
		labels := cloneStringMap(f.labelsBy[id])
		if !matchLabelFilters(labels, filters) {
			return RuntimeContainer{}, false
		}
		vm, exists := vmByID[id]
		if !exists || vm.Status == firecrackerrt.StatusStopped {
			return RuntimeContainer{}, false
		}
		return RuntimeContainer{
			ID:     id,
			Names:  []string{id},
			Image:  f.imageBy[id],
			Labels: labels,
		}, true
	}), nil
}
