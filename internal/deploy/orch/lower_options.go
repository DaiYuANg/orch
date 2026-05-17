package orch

import (
	"maps"

	"github.com/arcgolabs/mapper"
	"github.com/arcgolabs/plano/compiler"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func lowerDockerOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.DockerOptions {
	docker := mapHIRFields[v1.DockerOptions](m, f)
	if docker.NetworkMode == "" {
		if network, ok := stringField(f, "network"); ok {
			docker.NetworkMode = network
		}
	}
	return nonEmptyDockerOptions(docker)
}

func lowerContainerdOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.ContainerdOptions {
	containerd := mapHIRFields[v1.ContainerdOptions](m, f)
	if containerd.Namespace == "" {
		return nil
	}
	return &containerd
}

func lowerFirecrackerOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.FirecrackerOptions {
	firecracker := mapHIRFields[v1.FirecrackerOptions](m, f)
	if firecrackerOptionsEmpty(firecracker) {
		return nil
	}
	return &firecracker
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

func lowerProcessOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.ProcessOptions {
	process := mapHIRFields[v1.ProcessOptions](m, f)
	if process.GracefulStopTimeout == "" && process.StdoutPath == "" && process.StderrPath == "" {
		return nil
	}
	return &process
}

func lowerSystemdOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.SystemdOptions {
	systemd := mapHIRFields[v1.SystemdOptions](m, f)
	if stringFieldsEmpty(systemd.UnitName, systemd.User, systemd.Group, systemd.Restart, systemd.RestartSec, systemd.WantedBy) {
		return nil
	}
	return &systemd
}

func lowerWindowsServiceOptions(m *mapper.Mapper, f *compiler.HIRForm) *v1.WindowsServiceOptions {
	windowsService := mapHIRFields[v1.WindowsServiceOptions](m, f)
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
	if runtime != v1.RuntimeDocker && runtime != v1.RuntimePodman {
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
