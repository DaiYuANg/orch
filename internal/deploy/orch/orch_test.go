package orch

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCompileAndLowerSample(t *testing.T) {
	c, err := NewCompiler()
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrch(c)
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`app {
  metadata {
    name = "demo"
    namespace = "default"
  }
  workload web {
    kind = "service"
    runtime = "containerd"
    replicas = 2
    run {
      image = "nginx:alpine"
    }
    endpoint http {
      port = 80
      protocol = "http"
    }
  }
}
`)
	res := c.CompileSourceDetailed(context.Background(), "sample.orch", src)
	if res.Diagnostics.HasError() {
		t.Fatalf("compile: %v", res.Diagnostics)
	}
	app, err := orch.LowerHIR(res.HIR)
	if err != nil {
		t.Fatal(err)
	}
	if app.Metadata.Name != "demo" {
		t.Fatalf("metadata.name = %q", app.Metadata.Name)
	}
	if len(app.Workloads) != 1 || app.Workloads[0].Name != "web" {
		t.Fatalf("workloads = %#v", app.Workloads)
	}
	if app.Workloads[0].Run.Image != "nginx:alpine" {
		t.Fatalf("image = %q", app.Workloads[0].Run.Image)
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOrchLoadAppFromString(t *testing.T) {
	c, err := NewCompiler()
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrch(c)
	if err != nil {
		t.Fatal(err)
	}
	src := `app {
  metadata { name = "mem" }
  workload w {
    kind = "service"
    runtime = "docker"
    run { image = "alpine:latest" }
  }
}`
	ctx := context.Background()
	app, err := orch.LoadAppString(ctx, "inline.orch", src)
	if err != nil {
		t.Fatal(err)
	}
	if app.Metadata.Name != "mem" {
		t.Fatalf("name = %q", app.Metadata.Name)
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOrchLoadFullstackDockerExample(t *testing.T) {
	c, err := NewCompiler()
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrch(c)
	if err != nil {
		t.Fatal(err)
	}
	app, err := orch.LoadAppFile(context.Background(), filepath.FromSlash("../../../examples/fullstack-docker.orch"))
	if err != nil {
		t.Fatal(err)
	}
	if app.Metadata.Name != "fullstack" || app.Metadata.Namespace != "demo" {
		t.Fatalf("metadata = %+v", app.Metadata)
	}
	if len(app.Workloads) != 4 {
		t.Fatalf("workloads = %d, want 4", len(app.Workloads))
	}
	backend := app.Workloads[2]
	if backend.Name != "backend" {
		t.Fatalf("backend workload order/name = %q", backend.Name)
	}
	if backend.Run.Options.Docker == nil || backend.Run.Options.Docker.NetworkMode != "orch-demo" {
		t.Fatalf("backend docker options = %+v", backend.Run.Options.Docker)
	}
	if len(backend.DependsOn) != 2 || backend.DependsOn[0].Name != "postgres" || backend.DependsOn[1].Name != "redis" {
		t.Fatalf("backend dependsOn = %+v", backend.DependsOn)
	}
	if app.Workloads[0].Scheduling == nil || !app.Workloads[0].Scheduling.Stateful {
		t.Fatalf("postgres scheduling = %+v", app.Workloads[0].Scheduling)
	}
	if len(app.Ingresses) != 1 || len(app.Ingresses[0].Routes) != 2 {
		t.Fatalf("ingresses = %+v", app.Ingresses)
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}
