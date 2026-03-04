package task

import (
	"fmt"
	"strings"
)

const (
	driverDocker         = "docker"
	driverContainerd     = "containerd"
	driverSystemd        = "systemd"
	driverFirecracker    = "firecracker"
	driverWindowsService = "windows-service"
)

func newRuntimeFactories(primary RuntimeFactory) map[string]RuntimeFactory {
	dockerFactory := primary
	if dockerFactory == nil {
		dockerFactory = newDockerRuntimeExecutor
	}

	return map[string]RuntimeFactory{
		driverDocker:         dockerFactory,
		driverContainerd:     newContainerdRuntimeExecutor,
		driverSystemd:        newSystemdRuntimeExecutor,
		driverFirecracker:    newFirecrackerRuntimeExecutor,
		driverWindowsService: newWindowsServiceRuntimeExecutor,
	}
}

func unsupportedRuntimeFactory(driver string) RuntimeFactory {
	return func() (RuntimeExecutor, error) {
		return nil, fmt.Errorf("runtime driver %q is not implemented yet", driver)
	}
}

func normalizeRuntimeDriver(driver string) string {
	switch normalized := trimLower(driver); normalized {
	case "windowsservice", "windows_service":
		return driverWindowsService
	case "fire-cracker":
		return driverFirecracker
	case "":
		return driverDocker
	default:
		return normalized
	}
}

func trimLower(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
