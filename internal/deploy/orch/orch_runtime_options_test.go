package orch_test

import (
	"testing"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func TestOrchProcessShortForm(t *testing.T) {
	app := loadAppString(t, "process.orch", `app {
  name = "process-demo"

  worker local {
    runtime = "process"
    command = ["/opt/app/worker"]
    args = ["--once"]
    cwd = "/opt/app"

    process {
      graceful_stop_timeout = "2s"
    }
  }
}`)
	workload := workloadByName(t, app, "local")
	requireKindRuntime(t, workload, v1.WorkloadKindWorker, v1.RuntimeProcess)
	requireProcessRun(t, workload)
	requireValidApp(t, app)
}

func TestOrchFirecrackerOptions(t *testing.T) {
	app := loadAppString(t, "firecracker.orch", `app {
  name = "vm-demo"

  service vm {
    runtime = "firecracker"

    firecracker {
      kernel_image_path = "/var/lib/orch/vmlinux"
      rootfs_path = "/var/lib/orch/rootfs.ext4"
      boot_args = "console=ttyS0 root=/dev/vda rw"
      binary_path = "/usr/local/bin/firecracker"
      socket_path = "/run/orch/firecracker-vm.sock"
      rootfs_read_only = true
      network_interface_id = "eth0"
      tap_device_name = "tap-orch0"
      guest_mac = "AA:FC:00:00:00:01"
      allow_mmds_requests = true
      vcpu_count = 2
      mem_size_mib = 256
    }
  }
}`)
	workload := workloadByName(t, app, "vm")
	requireKindRuntime(t, workload, v1.WorkloadKindService, v1.RuntimeFirecracker)
	requireFirecrackerOptions(t, workload.Run.Options.Firecracker)
	requireValidApp(t, app)
}

func TestOrchPodmanOptions(t *testing.T) {
	app := loadAppString(t, "podman.orch", `app {
  name = "runtime-podman"

  service api {
    runtime = "podman"
    image = "alpine:3.21"
    podman {
      network = "podman-net"
      privileged = true
    }
  }
}`)
	workload := workloadByName(t, app, "api")
	requireKindRuntime(t, workload, v1.WorkloadKindService, v1.RuntimePodman)
	requireDockerNetwork(t, workload, "podman-net")
	if workload.Run.Options.Docker == nil || !workload.Run.Options.Docker.Privileged {
		t.Fatalf("docker options = %+v", workload.Run.Options.Docker)
	}
	requireValidApp(t, app)
}

func TestOrchPodmanDefaultAndOverrideDockerOptions(t *testing.T) {
	app := loadAppString(t, "podman-defaults.orch", `app {
  name = "runtime-podman-defaults"
  runtime = "podman"

  docker {
    network = "global-net"
  }

  service api {
    image = "alpine:3.21"
    podman {
      privileged = true
    }
  }
}`)
	workload := workloadByName(t, app, "api")
	requireKindRuntime(t, workload, v1.WorkloadKindService, v1.RuntimePodman)
	requireDockerNetwork(t, workload, "global-net")
	if workload.Run.Options.Docker == nil || !workload.Run.Options.Docker.Privileged {
		t.Fatalf("docker options = %+v", workload.Run.Options.Docker)
	}
	requireValidApp(t, app)
}

func requireProcessRun(t *testing.T, workload v1.Workload) {
	t.Helper()
	if len(workload.Run.Exec.Command) != 1 || workload.Run.Exec.Command[0] != "/opt/app/worker" {
		t.Fatalf("command = %#v", workload.Run.Exec.Command)
	}
	if len(workload.Run.Exec.Args) != 1 || workload.Run.Exec.Args[0] != "--once" {
		t.Fatalf("args = %#v", workload.Run.Exec.Args)
	}
	if workload.Run.Cwd != "/opt/app" {
		t.Fatalf("cwd = %q", workload.Run.Cwd)
	}
	if workload.Run.Options.Process == nil || workload.Run.Options.Process.GracefulStopTimeout != "2s" {
		t.Fatalf("process options = %+v", workload.Run.Options.Process)
	}
}

func requireFirecrackerOptions(t *testing.T, fc *v1.FirecrackerOptions) {
	t.Helper()
	if fc == nil {
		t.Fatal("firecracker options are nil")
	}
	requireFirecrackerPaths(t, fc)
	requireFirecrackerAdvanced(t, fc)
	requireFirecrackerNetwork(t, fc)
	requireFirecrackerMachine(t, fc)
}

func requireFirecrackerPaths(t *testing.T, fc *v1.FirecrackerOptions) {
	t.Helper()
	if fc.KernelImagePath != "/var/lib/orch/vmlinux" || fc.RootfsPath != "/var/lib/orch/rootfs.ext4" {
		t.Fatalf("paths = %+v", fc)
	}
}

func requireFirecrackerAdvanced(t *testing.T, fc *v1.FirecrackerOptions) {
	t.Helper()
	if fc.BootArgs != "console=ttyS0 root=/dev/vda rw" || fc.BinaryPath != "/usr/local/bin/firecracker" || fc.SocketPath != "/run/orch/firecracker-vm.sock" {
		t.Fatalf("advanced options = %+v", fc)
	}
}

func requireFirecrackerNetwork(t *testing.T, fc *v1.FirecrackerOptions) {
	t.Helper()
	if fc.NetworkInterfaceID != "eth0" || fc.TapDeviceName != "tap-orch0" || fc.GuestMAC != "AA:FC:00:00:00:01" || !fc.AllowMMDSRequests {
		t.Fatalf("network options = %+v", fc)
	}
}

func requireFirecrackerMachine(t *testing.T, fc *v1.FirecrackerOptions) {
	t.Helper()
	if !fc.RootfsReadOnly || fc.VCPUCount != 2 || fc.MemSizeMiB != 256 {
		t.Fatalf("machine options = %+v", fc)
	}
}
