package systemd

import (
	"strings"
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestDefaultUnitName(t *testing.T) {
	t.Parallel()

	meta := deployv1.Metadata{Name: "Demo_App", Namespace: ""}
	if got := defaultUnitName(meta, "api"); got != "orch-default-demo_app-api.service" {
		t.Fatalf("unit name = %q", got)
	}
	if got := normalizeUnitName("custom-api"); got != "custom-api.service" {
		t.Fatalf("custom unit name = %q", got)
	}
}

func TestRenderUnit(t *testing.T) {
	t.Parallel()

	meta := deployv1.Metadata{Name: "demo", Namespace: "prod"}
	workload := deployv1.Workload{
		Name: "api",
		Kind: deployv1.WorkloadKindService,
		Run: deployv1.RunSpec{
			Exec: deployv1.ExecSpec{
				Command: []string{"/usr/local/bin/api", "--listen"},
				Args:    []string{":8080"},
			},
			Env: []deployv1.EnvVar{
				{Name: "APP_ENV", Value: "prod canary"},
			},
			Cwd:  "/srv/demo api",
			User: "orch",
			Options: deployv1.RunOptions{
				Systemd: &deployv1.SystemdOptions{
					Group:      "orch",
					RestartSec: "5s",
				},
			},
		},
	}

	unit, err := renderUnit(meta, workload, "orch-prod-demo-api.service")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Description=orch workload prod/demo/api",
		"SyslogIdentifier=orch-prod-demo-api",
		"WorkingDirectory=\"/srv/demo api\"",
		"User=orch",
		"Group=orch",
		"Environment=\"APP_ENV=prod canary\"",
		"ExecStart=\"/usr/local/bin/api\" \"--listen\" \":8080\"",
		"Restart=on-failure",
		"RestartSec=5s",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q\n%s", want, unit)
		}
	}
}
