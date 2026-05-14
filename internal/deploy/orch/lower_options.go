package orch

import (
	"maps"
	"strings"

	"github.com/arcgolabs/plano/compiler"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func lowerDockerOptions(f *compiler.HIRForm) *v1.DockerOptions {
	var docker v1.DockerOptions
	if network, ok := stringField(f, "network"); ok {
		docker.NetworkMode = strings.TrimSpace(network)
	}
	if networkMode, ok := stringField(f, "network_mode"); ok {
		docker.NetworkMode = strings.TrimSpace(networkMode)
	}
	if privileged, ok := boolField(f, "privileged"); ok {
		docker.Privileged = privileged
	}
	if labels, ok := stringMapField(f, "labels"); ok {
		docker.Labels = labels
	}
	return nonEmptyDockerOptions(docker)
}

func lowerContainerdOptions(f *compiler.HIRForm) *v1.ContainerdOptions {
	var containerd v1.ContainerdOptions
	if ns, ok := stringField(f, "namespace"); ok {
		containerd.Namespace = strings.TrimSpace(ns)
	}
	if containerd.Namespace == "" {
		return nil
	}
	return &containerd
}

func lowerFirecrackerOptions(f *compiler.HIRForm) *v1.FirecrackerOptions {
	var firecracker v1.FirecrackerOptions
	fillFirecrackerPathOptions(&firecracker, f)
	fillFirecrackerNetworkOptions(&firecracker, f)
	fillFirecrackerSizingOptions(&firecracker, f)
	if firecrackerOptionsEmpty(firecracker) {
		return nil
	}
	return &firecracker
}

func fillFirecrackerPathOptions(fc *v1.FirecrackerOptions, f *compiler.HIRForm) {
	if kernel, ok := stringField(f, "kernel_image_path"); ok {
		fc.KernelImagePath = strings.TrimSpace(kernel)
	}
	if rootfs, ok := stringField(f, "rootfs_path"); ok {
		fc.RootfsPath = strings.TrimSpace(rootfs)
	}
	if bootArgs, ok := stringField(f, "boot_args"); ok {
		fc.BootArgs = strings.TrimSpace(bootArgs)
	}
	if binaryPath, ok := stringField(f, "binary_path"); ok {
		fc.BinaryPath = strings.TrimSpace(binaryPath)
	}
	if socketPath, ok := stringField(f, "socket_path"); ok {
		fc.SocketPath = strings.TrimSpace(socketPath)
	}
	if rootfsReadOnly, ok := boolField(f, "rootfs_read_only"); ok {
		fc.RootfsReadOnly = rootfsReadOnly
	}
}

func fillFirecrackerNetworkOptions(fc *v1.FirecrackerOptions, f *compiler.HIRForm) {
	if ifaceID, ok := stringField(f, "network_interface_id"); ok {
		fc.NetworkInterfaceID = strings.TrimSpace(ifaceID)
	}
	if tapDevice, ok := stringField(f, "tap_device_name"); ok {
		fc.TapDeviceName = strings.TrimSpace(tapDevice)
	}
	if guestMAC, ok := stringField(f, "guest_mac"); ok {
		fc.GuestMAC = strings.TrimSpace(guestMAC)
	}
	if allowMMDS, ok := boolField(f, "allow_mmds_requests"); ok {
		fc.AllowMMDSRequests = allowMMDS
	}
}

func fillFirecrackerSizingOptions(fc *v1.FirecrackerOptions, f *compiler.HIRForm) {
	if vcpu, ok := intField(f, "vcpu_count"); ok {
		fc.VCPUCount = vcpu
	}
	if mem, ok := intField(f, "mem_size_mib"); ok {
		fc.MemSizeMiB = mem
	}
}

func firecrackerOptionsEmpty(fc v1.FirecrackerOptions) bool {
	for _, value := range []string{fc.KernelImagePath, fc.RootfsPath, fc.BootArgs, fc.BinaryPath, fc.SocketPath, fc.NetworkInterfaceID, fc.TapDeviceName, fc.GuestMAC} {
		if value != "" {
			return false
		}
	}
	if fc.RootfsReadOnly || fc.AllowMMDSRequests {
		return false
	}
	return fc.VCPUCount == 0 && fc.MemSizeMiB == 0
}

func lowerProcessOptions(f *compiler.HIRForm) *v1.ProcessOptions {
	var process v1.ProcessOptions
	if timeout, ok := stringField(f, "graceful_stop_timeout"); ok {
		process.GracefulStopTimeout = strings.TrimSpace(timeout)
	}
	if stdoutPath, ok := stringField(f, "stdout_path"); ok {
		process.StdoutPath = strings.TrimSpace(stdoutPath)
	}
	if stderrPath, ok := stringField(f, "stderr_path"); ok {
		process.StderrPath = strings.TrimSpace(stderrPath)
	}
	if process.GracefulStopTimeout == "" && process.StdoutPath == "" && process.StderrPath == "" {
		return nil
	}
	return &process
}

func lowerSystemdOptions(f *compiler.HIRForm) *v1.SystemdOptions {
	var systemd v1.SystemdOptions
	if unit, ok := stringField(f, "unit_name"); ok {
		systemd.UnitName = strings.TrimSpace(unit)
	}
	if user, ok := stringField(f, "user"); ok {
		systemd.User = strings.TrimSpace(user)
	}
	if group, ok := stringField(f, "group"); ok {
		systemd.Group = strings.TrimSpace(group)
	}
	if restart, ok := stringField(f, "restart"); ok {
		systemd.Restart = strings.TrimSpace(restart)
	}
	if restartSec, ok := stringField(f, "restart_sec"); ok {
		systemd.RestartSec = strings.TrimSpace(restartSec)
	}
	if wantedBy, ok := stringField(f, "wanted_by"); ok {
		systemd.WantedBy = strings.TrimSpace(wantedBy)
	}
	if stringFieldsEmpty(systemd.UnitName, systemd.User, systemd.Group, systemd.Restart, systemd.RestartSec, systemd.WantedBy) {
		return nil
	}
	return &systemd
}

func lowerWindowsServiceOptions(f *compiler.HIRForm) *v1.WindowsServiceOptions {
	var windowsService v1.WindowsServiceOptions
	if serviceName, ok := stringField(f, "service_name"); ok {
		windowsService.ServiceName = strings.TrimSpace(serviceName)
	}
	if displayName, ok := stringField(f, "display_name"); ok {
		windowsService.DisplayName = strings.TrimSpace(displayName)
	}
	if startType, ok := stringField(f, "start_type"); ok {
		windowsService.StartType = strings.TrimSpace(startType)
	}
	if stringFieldsEmpty(windowsService.ServiceName, windowsService.DisplayName, windowsService.StartType) {
		return nil
	}
	return &windowsService
}

func stringFieldsEmpty(values ...string) bool {
	for _, value := range values {
		if value != "" {
			return false
		}
	}
	return true
}

func mergeDockerOptionsForRuntime(runtime v1.RuntimeKind, base, override *v1.DockerOptions) *v1.DockerOptions {
	if runtime != v1.RuntimeDocker {
		return override
	}
	return mergeDockerOptions(base, override)
}

func mergeDockerOptions(base, override *v1.DockerOptions) *v1.DockerOptions {
	if base == nil && override == nil {
		return nil
	}
	out := cloneDockerOptions(base)
	applyDockerOverride(&out, override)
	return nonEmptyDockerOptions(out)
}

func cloneDockerOptions(base *v1.DockerOptions) v1.DockerOptions {
	if base == nil {
		return v1.DockerOptions{}
	}
	out := *base
	if base.Labels != nil {
		out.Labels = cloneStringMap(base.Labels)
	}
	return out
}

func applyDockerOverride(out, override *v1.DockerOptions) {
	if override == nil {
		return
	}
	if override.NetworkMode != "" {
		out.NetworkMode = override.NetworkMode
	}
	if override.Privileged {
		out.Privileged = true
	}
	if len(override.Labels) > 0 {
		if out.Labels == nil {
			out.Labels = map[string]string{}
		}
		maps.Copy(out.Labels, override.Labels)
	}
}

func nonEmptyDockerOptions(docker v1.DockerOptions) *v1.DockerOptions {
	if docker.NetworkMode == "" && !docker.Privileged && len(docker.Labels) == 0 {
		return nil
	}
	return &docker
}
