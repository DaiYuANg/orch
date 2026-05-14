package v1alpha1

import (
	"net"
	"strings"

	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (w *Workload) validateRunForRuntime() error {
	switch w.Runtime {
	case RuntimeDocker, RuntimeContainerd:
		return w.validateImageRuntime()
	case RuntimeFirecracker:
		return w.validateFirecrackerRuntime()
	case RuntimeProcess, RuntimeSystemd, RuntimeWindowsService:
		return w.validateExecRuntime()
	}
	return nil
}

func (w *Workload) validateImageRuntime() error {
	if strings.TrimSpace(w.Run.Artifact.Image) == "" {
		return oopsx.B("deploy").Errorf("run.artifact.image is required for runtime %q", w.Runtime)
	}
	return nil
}

func (w *Workload) validateFirecrackerRuntime() error {
	opts := w.Run.Options.Firecracker
	if opts == nil {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker is required for runtime %q", w.Runtime)
	}
	validators := []func(*FirecrackerOptions) error{
		validateFirecrackerKernel,
		validateFirecrackerRootfs,
		validateFirecrackerNetwork,
		validateFirecrackerCapacity,
	}
	for _, validate := range validators {
		if err := validate(opts); err != nil {
			return err
		}
	}
	return nil
}

func validateFirecrackerKernel(opts *FirecrackerOptions) error {
	if strings.TrimSpace(opts.KernelImagePath) == "" {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.kernelImagePath is required")
	}
	return nil
}

func validateFirecrackerRootfs(opts *FirecrackerOptions) error {
	if strings.TrimSpace(opts.RootfsPath) == "" {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.rootfsPath is required")
	}
	return nil
}

func validateFirecrackerCapacity(opts *FirecrackerOptions) error {
	if opts.VCPUCount < 0 {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.vcpuCount must be >= 0")
	}
	if opts.MemSizeMiB < 0 {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.memSizeMiB must be >= 0")
	}
	return nil
}

func (w *Workload) validateExecRuntime() error {
	if len(w.Run.Exec.Command) == 0 && strings.TrimSpace(w.Run.Artifact.Path) == "" {
		return oopsx.B("deploy").Errorf("run.exec.command or run.artifact.path is required for runtime %q", w.Runtime)
	}
	return nil
}

func validateFirecrackerNetwork(opts *FirecrackerOptions) error {
	if opts == nil {
		return nil
	}
	if !hasFirecrackerNetworkFields(opts) {
		return nil
	}
	if strings.TrimSpace(opts.TapDeviceName) == "" {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.tapDeviceName is required when firecracker network is configured")
	}
	return validateFirecrackerGuestMAC(opts)
}

func hasFirecrackerNetworkFields(opts *FirecrackerOptions) bool {
	return strings.TrimSpace(opts.NetworkInterfaceID) != "" ||
		strings.TrimSpace(opts.TapDeviceName) != "" ||
		strings.TrimSpace(opts.GuestMAC) != "" ||
		opts.AllowMMDSRequests
}

func validateFirecrackerGuestMAC(opts *FirecrackerOptions) error {
	mac := strings.TrimSpace(opts.GuestMAC)
	if mac == "" {
		return nil
	}
	if _, err := net.ParseMAC(mac); err != nil {
		return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.guestMAC is invalid: %q", mac)
	}
	return nil
}
