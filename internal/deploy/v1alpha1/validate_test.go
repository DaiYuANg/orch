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
				Run:     RunSpec{Image: "nginx"},
			},
		},
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
