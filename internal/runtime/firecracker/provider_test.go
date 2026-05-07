package firecracker

import (
	"path/filepath"
	"strings"
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestBuildConfigDefaults(t *testing.T) {
	t.Parallel()

	provider := &Provider{root: t.TempDir()}
	meta := deployv1.Metadata{Name: "demo", Namespace: "prod"}
	workload := deployv1.Workload{
		Name: "vm",
		Run: deployv1.RunSpec{
			Options: deployv1.RunOptions{
				Firecracker: &deployv1.FirecrackerOptions{
					KernelImagePath: "/var/lib/orch/vmlinux",
					RootfsPath:      "/var/lib/orch/rootfs.ext4",
				},
			},
		},
	}

	cfg, err := provider.buildConfig(meta, workload)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ID != "prod-demo-vm" {
		t.Fatalf("id = %q", cfg.ID)
	}
	if cfg.BinaryPath != defaultBinaryPath {
		t.Fatalf("binary = %q", cfg.BinaryPath)
	}
	if cfg.VCPUCount != defaultVCPUCount || cfg.MemSizeMiB != defaultMemSizeMiB {
		t.Fatalf("machine config = %d/%d", cfg.VCPUCount, cfg.MemSizeMiB)
	}
	if !strings.Contains(cfg.BootArgs, "root=/dev/vda") {
		t.Fatalf("boot args = %q", cfg.BootArgs)
	}
	if !strings.HasSuffix(cfg.BootArgs, " rw") {
		t.Fatalf("boot args = %q, want rw rootfs", cfg.BootArgs)
	}
	if !strings.HasSuffix(cfg.APISocket, filepath.Join("sockets", "prod-demo-vm.sock")) {
		t.Fatalf("api socket = %q", cfg.APISocket)
	}
}

func TestBuildConfigOptions(t *testing.T) {
	t.Parallel()

	provider := &Provider{root: t.TempDir()}
	workload := deployv1.Workload{
		Name: "vm",
		Run: deployv1.RunSpec{
			Options: deployv1.RunOptions{
				Firecracker: &deployv1.FirecrackerOptions{
					KernelImagePath: "/kernel",
					RootfsPath:      "/rootfs",
					BootArgs:        "console=ttyS0 root=/dev/vda ro",
					BinaryPath:      "/usr/local/bin/firecracker",
					SocketPath:      filepath.Join(t.TempDir(), "fc.sock"),
					RootfsReadOnly:  true,
					TapDeviceName:   "tap-orch0",
					GuestMAC:        "AA:FC:00:00:00:01",
					VCPUCount:       2,
					MemSizeMiB:      256,
				},
			},
		},
	}

	cfg, err := provider.buildConfig(deployv1.Metadata{Name: "demo"}, workload)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BinaryPath != "/usr/local/bin/firecracker" || cfg.VCPUCount != 2 || cfg.MemSizeMiB != 256 {
		t.Fatalf("config = %+v", cfg)
	}
	if !cfg.RootfsReadOnly || cfg.BootArgs != "console=ttyS0 root=/dev/vda ro" {
		t.Fatalf("rootfs/boot config = %+v", cfg)
	}
	if cfg.Network == nil || cfg.Network.InterfaceID != "eth0" || cfg.Network.TapDeviceName != "tap-orch0" || cfg.Network.GuestMAC != "AA:FC:00:00:00:01" {
		t.Fatalf("network config = %+v", cfg.Network)
	}
}

func TestBuildConfigReadOnlyDefaultBootArgs(t *testing.T) {
	t.Parallel()

	provider := &Provider{root: t.TempDir()}
	workload := deployv1.Workload{
		Name: "vm",
		Run: deployv1.RunSpec{
			Options: deployv1.RunOptions{
				Firecracker: &deployv1.FirecrackerOptions{
					KernelImagePath: "/kernel",
					RootfsPath:      "/rootfs",
					RootfsReadOnly:  true,
				},
			},
		},
	}

	cfg, err := provider.buildConfig(deployv1.Metadata{Name: "demo"}, workload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.BootArgs, " ro") {
		t.Fatalf("boot args = %q, want ro rootfs", cfg.BootArgs)
	}
}

func TestBuildConfigNetworkRequiresTapDevice(t *testing.T) {
	t.Parallel()

	provider := &Provider{root: t.TempDir()}
	workload := deployv1.Workload{
		Name: "vm",
		Run: deployv1.RunSpec{
			Options: deployv1.RunOptions{
				Firecracker: &deployv1.FirecrackerOptions{
					KernelImagePath:    "/kernel",
					RootfsPath:         "/rootfs",
					NetworkInterfaceID: "eth0",
				},
			},
		},
	}

	_, err := provider.buildConfig(deployv1.Metadata{Name: "demo"}, workload)
	if err == nil || !strings.Contains(err.Error(), "tap_device_name is required") {
		t.Fatalf("buildConfig error = %v, want tap device error", err)
	}
}

func TestFirecrackerArtifactSummaryFallsBackToRootfs(t *testing.T) {
	t.Parallel()

	run := deployv1.RunSpec{
		Options: deployv1.RunOptions{
			Firecracker: &deployv1.FirecrackerOptions{RootfsPath: "/rootfs.ext4"},
		},
	}
	if got := firecrackerArtifactSummary(run); got != "/rootfs.ext4" {
		t.Fatalf("summary = %q", got)
	}
}
