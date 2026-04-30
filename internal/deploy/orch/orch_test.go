package orch

import (
	"context"
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
