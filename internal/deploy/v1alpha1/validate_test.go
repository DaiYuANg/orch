package v1alpha1

import (
	"strings"
	"testing"
)

func validApp() *App {
	return &App{
		Metadata: Metadata{Name: "demo"},
		Workloads: []Workload{
			{
				Name:    "api",
				Kind:    WorkloadKindService,
				Runtime: RuntimeDocker,
				Run:     RunSpec{Artifact: ArtifactSpec{Image: "nginx"}},
			},
		},
	}
}

func TestValidateProcessRequiresCommandOrArtifactPath(t *testing.T) {
	app := validApp()
	app.Workloads[0].Runtime = RuntimeProcess
	app.Workloads[0].Run.Artifact = ArtifactSpec{}

	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), `run.exec.command or run.artifact.path is required for runtime "process"`) {
		t.Fatalf("Validate() error = %v, want process command error", err)
	}

	app.Workloads[0].Run.Artifact.Path = "/opt/app/api"
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() with artifact path = %v", err)
	}
}

func TestValidateFirecrackerRequiresRuntimeOptions(t *testing.T) {
	app := validApp()
	app.Workloads[0].Runtime = RuntimeFirecracker
	app.Workloads[0].Run.Artifact = ArtifactSpec{}

	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), `run.runtimeOptions.firecracker is required`) {
		t.Fatalf("Validate() error = %v, want firecracker options error", err)
	}

	app.Workloads[0].Run.Options.Firecracker = &FirecrackerOptions{
		KernelImagePath: "/var/lib/orch/vmlinux",
		RootfsPath:      "/var/lib/orch/rootfs.ext4",
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() with firecracker options = %v", err)
	}

	app.Workloads[0].Run.Options.Firecracker.MemSizeMiB = -1
	err = app.Validate()
	if err == nil || !strings.Contains(err.Error(), `run.runtimeOptions.firecracker.memSizeMiB must be >= 0`) {
		t.Fatalf("Validate() error = %v, want firecracker mem error", err)
	}
}

func TestValidateFirecrackerNetwork(t *testing.T) {
	app := validApp()
	app.Workloads[0].Runtime = RuntimeFirecracker
	app.Workloads[0].Run.Artifact = ArtifactSpec{}
	app.Workloads[0].Run.Options.Firecracker = &FirecrackerOptions{
		KernelImagePath:    "/var/lib/orch/vmlinux",
		RootfsPath:         "/var/lib/orch/rootfs.ext4",
		NetworkInterfaceID: "eth0",
	}

	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), `tapDeviceName is required`) {
		t.Fatalf("Validate() error = %v, want tap device error", err)
	}

	app.Workloads[0].Run.Options.Firecracker.TapDeviceName = "tap-orch0"
	app.Workloads[0].Run.Options.Firecracker.GuestMAC = "not-a-mac"
	err = app.Validate()
	if err == nil || !strings.Contains(err.Error(), `guestMAC is invalid`) {
		t.Fatalf("Validate() error = %v, want mac error", err)
	}

	app.Workloads[0].Run.Options.Firecracker.GuestMAC = "AA:FC:00:00:00:01"
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() with firecracker network = %v", err)
	}
}

func TestValidateRejectsEmptyEnvName(t *testing.T) {
	app := validApp()
	app.Workloads[0].Run.Env = []EnvVar{{Name: " ", Value: "8080"}}

	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), "run.env[0].name is required") {
		t.Fatalf("Validate() error = %v, want empty env name error", err)
	}
}

func TestValidateRejectsNegativeResources(t *testing.T) {
	tests := []struct {
		name string
		res  Resources
		want string
	}{
		{name: "cpu", res: Resources{CPUMillis: -1}, want: "resources.cpuMillis must be >= 0"},
		{name: "memory", res: Resources{MemoryBytes: -1}, want: "resources.memoryBytes must be >= 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := validApp()
			app.Workloads[0].Resources = &tt.res

			err := app.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsWorkloadDependencyCycle(t *testing.T) {
	app := validApp()
	app.Workloads = append(app.Workloads, Workload{
		Name:    "db",
		Kind:    WorkloadKindStateful,
		Runtime: RuntimeDocker,
		Run:     RunSpec{Artifact: ArtifactSpec{Image: "postgres"}},
	})
	app.Workloads[0].DependsOn = []WorkloadRef{{Name: "db"}}
	app.Workloads[1].DependsOn = []WorkloadRef{{Name: "api"}}

	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), "workloads dependsOn contains a cycle") {
		t.Fatalf("Validate() error = %v, want dependency cycle error", err)
	}
}

func TestValidateAllowsAcyclicWorkloadDependencies(t *testing.T) {
	app := validApp()
	app.Workloads = append(app.Workloads, Workload{
		Name:    "db",
		Kind:    WorkloadKindStateful,
		Runtime: RuntimeDocker,
		Run:     RunSpec{Artifact: ArtifactSpec{Image: "postgres"}},
	})
	app.Workloads[0].DependsOn = []WorkloadRef{{Name: "db"}}

	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() with acyclic dependencies = %v", err)
	}
}
