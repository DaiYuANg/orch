package orch_test

import (
	"context"
	"testing"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func TestCompileAndLowerSample(t *testing.T) {
	c, orch := newTestOrch(t)
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
	requireMetadata(t, app, "demo", "default")
	requireWorkloadCount(t, app, 1)
	web := workloadByName(t, app, "web")
	requireKindRuntime(t, web, v1.WorkloadKindService, v1.RuntimeContainerd)
	if web.Run.Artifact.Image != "nginx:alpine" {
		t.Fatalf("image = %q", web.Run.Artifact.Image)
	}
	requireValidApp(t, app)
}

func TestOrchLoadAppFromString(t *testing.T) {
	app := loadAppString(t, "inline.orch", `app {
  metadata { name = "mem" }
  workload w {
    kind = "service"
    runtime = "docker"
    run { image = "alpine:latest" }
  }
}`)
	requireMetadata(t, app, "mem", "")
	requireValidApp(t, app)
}
