package runtime

import (
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
)

const (
	containerdSocket  = "/run/containerd/containerd.sock"
	dockerSocket      = "/var/run/docker.sock"
	podmanRootSocket  = "/run/podman/podman.sock"
	systemdRuntimeDir = "/run/systemd/system"
)

type Environment struct {
	OS             string
	Docker         bool
	Podman         bool
	Containerd     bool
	Firecracker    bool
	Process        bool
	Systemd        bool
	WindowsService bool
}

func DetectEnvironment() Environment {
	return detectEnvironment(defaultEnvironmentProbe{})
}

type environmentProbe interface {
	goos() string
	env(string) string
	exists(string) bool
	lookPath(string) bool
}

type defaultEnvironmentProbe struct{}

func (defaultEnvironmentProbe) goos() string {
	return goruntime.GOOS
}

func (defaultEnvironmentProbe) env(name string) string {
	return os.Getenv(name)
}

func (defaultEnvironmentProbe) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (defaultEnvironmentProbe) lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func detectEnvironment(probe environmentProbe) Environment {
	goos := strings.TrimSpace(probe.goos())
	if goos == "" {
		goos = goruntime.GOOS
	}
	return Environment{
		OS:             goos,
		Docker:         dockerAvailable(probe, goos),
		Podman:         podmanAvailable(probe, goos),
		Containerd:     goos == "linux" && probe.exists(containerdSocket),
		Firecracker:    goos == "linux" && probe.lookPath("firecracker"),
		Process:        true,
		Systemd:        goos == "linux" && (probe.exists(systemdRuntimeDir) || probe.lookPath("systemctl")),
		WindowsService: goos == "windows",
	}
}

func dockerAvailable(probe environmentProbe, goos string) bool {
	if strings.TrimSpace(probe.env("DOCKER_HOST")) != "" {
		return true
	}
	if probe.lookPath("docker") {
		return true
	}
	return goos == "linux" && probe.exists(dockerSocket)
}

func podmanAvailable(probe environmentProbe, goos string) bool {
	if strings.TrimSpace(probe.env("PODMAN_HOST")) != "" {
		return true
	}
	if strings.Contains(strings.ToLower(probe.env("DOCKER_HOST")), "podman") {
		return true
	}
	if probe.lookPath("podman") {
		return true
	}
	if goos != "linux" {
		return false
	}
	return probe.exists(podmanRootSocket) || probe.exists(userPodmanSocket(probe.env("XDG_RUNTIME_DIR")))
}

func userPodmanSocket(runtimeDir string) string {
	runtimeDir = strings.TrimSpace(runtimeDir)
	if runtimeDir == "" {
		return ""
	}
	return runtimeDir + "/podman/podman.sock"
}
