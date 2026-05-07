package orch

import (
	"context"
	"path/filepath"
	"testing"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
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

	postgres := workloadByName(t, app, "postgres")
	if postgres.Kind != v1.WorkloadKindStateful {
		t.Fatalf("postgres kind = %q", postgres.Kind)
	}
	if postgres.Scheduling == nil || !postgres.Scheduling.Stateful {
		t.Fatalf("postgres scheduling = %+v", postgres.Scheduling)
	}
	if postgres.Run.Options.Docker == nil || postgres.Run.Options.Docker.NetworkMode != "orch-demo" {
		t.Fatalf("postgres docker options = %+v", postgres.Run.Options.Docker)
	}
	if len(postgres.Endpoints) != 1 || postgres.Endpoints[0].Name != "tcp-5432" || postgres.Endpoints[0].Port != 5432 || postgres.Endpoints[0].Protocol != v1.ProtoTCP {
		t.Fatalf("postgres endpoints = %+v", postgres.Endpoints)
	}
	if postgres.Resources == nil || postgres.Resources.CPUMillis != 500 || postgres.Resources.MemoryBytes != 536870912 {
		t.Fatalf("postgres resources = %+v", postgres.Resources)
	}

	backend := workloadByName(t, app, "backend")
	if backend.Kind != v1.WorkloadKindService {
		t.Fatalf("backend kind = %q", backend.Kind)
	}
	if backend.Run.Options.Docker == nil || backend.Run.Options.Docker.NetworkMode != "orch-demo" {
		t.Fatalf("backend docker options = %+v", backend.Run.Options.Docker)
	}
	if len(backend.DependsOn) != 2 || backend.DependsOn[0].Name != "postgres" || backend.DependsOn[1].Name != "redis" {
		t.Fatalf("backend dependsOn = %+v", backend.DependsOn)
	}
	if len(backend.Endpoints) != 1 || backend.Endpoints[0].Name != "http" || backend.Endpoints[0].Port != 8080 || backend.Endpoints[0].Protocol != v1.ProtoHTTP {
		t.Fatalf("backend endpoints = %+v", backend.Endpoints)
	}
	if backend.Resources == nil || backend.Resources.CPUMillis != 500 || backend.Resources.MemoryBytes != 536870912 {
		t.Fatalf("backend resources = %+v", backend.Resources)
	}
	if got := envValue(backend, "HTTP_ADDR"); got != ":8080" {
		t.Fatalf("backend HTTP_ADDR = %q", got)
	}
	if len(app.Ingresses) != 1 || len(app.Ingresses[0].Routes) != 2 {
		t.Fatalf("ingresses = %+v", app.Ingresses)
	}
	apiRoute := app.Ingresses[0].Routes[0]
	if apiRoute.Path != "/api" || apiRoute.Backend.Workload != "backend" || apiRoute.Backend.Endpoint != "http" {
		t.Fatalf("api route = %+v", apiRoute)
	}
	frontendRoute := app.Ingresses[0].Routes[1]
	if frontendRoute.Path != "/" || frontendRoute.Backend.Workload != "frontend" || frontendRoute.Backend.Endpoint != "http" {
		t.Fatalf("frontend route = %+v", frontendRoute)
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOrchShortFormSugar(t *testing.T) {
	c, err := NewCompiler()
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrch(c)
	if err != nil {
		t.Fatal(err)
	}
	src := `app {
  name = "short"
  namespace = "demo"

  docker {
    network = "orch-demo"
  }

  stateful db {
    image = "postgres:16-alpine"
    env = {
      POSTGRES_DB = "app",
      POSTGRES_USER = "orch",
    }

    tcp(5432)
    resources = "1/1Gi"
  }

  service api {
    image = "ghcr.io/acme/api:latest"
    depends_on = [db]
    env = {
      DATABASE_URL = "postgres://db/app",
    }

    http(8080, "api")
    resources = "250m/256Mi"
  }

  ingress public {
    path "/" {
      workload = api
      endpoint = "api"
    }
  }
}`
	app, err := orch.LoadAppString(context.Background(), "short.orch", src)
	if err != nil {
		t.Fatal(err)
	}
	if app.Metadata.Name != "short" || app.Metadata.Namespace != "demo" {
		t.Fatalf("metadata = %+v", app.Metadata)
	}

	db := workloadByName(t, app, "db")
	if db.Kind != v1.WorkloadKindStateful || db.Runtime != v1.RuntimeDocker {
		t.Fatalf("db kind/runtime = %q/%q", db.Kind, db.Runtime)
	}
	if db.Scheduling == nil || !db.Scheduling.Stateful {
		t.Fatalf("db scheduling = %+v", db.Scheduling)
	}
	if db.Run.Options.Docker == nil || db.Run.Options.Docker.NetworkMode != "orch-demo" {
		t.Fatalf("db docker options = %+v", db.Run.Options.Docker)
	}
	if db.Resources == nil || db.Resources.CPUMillis != 1000 || db.Resources.MemoryBytes != 1073741824 {
		t.Fatalf("db resources = %+v", db.Resources)
	}

	api := workloadByName(t, app, "api")
	if len(api.DependsOn) != 1 || api.DependsOn[0].Name != "db" {
		t.Fatalf("api dependsOn = %+v", api.DependsOn)
	}
	if len(api.Endpoints) != 1 || api.Endpoints[0].Name != "api" || api.Endpoints[0].Port != 8080 || api.Endpoints[0].Protocol != v1.ProtoHTTP {
		t.Fatalf("api endpoints = %+v", api.Endpoints)
	}
	if api.Resources == nil || api.Resources.CPUMillis != 250 || api.Resources.MemoryBytes != 268435456 {
		t.Fatalf("api resources = %+v", api.Resources)
	}
	if got := envValue(api, "DATABASE_URL"); got != "postgres://db/app" {
		t.Fatalf("api DATABASE_URL = %q", got)
	}
	if len(app.Ingresses) != 1 || len(app.Ingresses[0].Routes) != 1 {
		t.Fatalf("ingresses = %+v", app.Ingresses)
	}
	route := app.Ingresses[0].Routes[0]
	if route.Path != "/" || route.Backend.Workload != "api" || route.Backend.Endpoint != "api" {
		t.Fatalf("route = %+v", route)
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

func workloadByName(t *testing.T, app *v1.App, name string) v1.Workload {
	t.Helper()
	for _, w := range app.Workloads {
		if w.Name == name {
			return w
		}
	}
	t.Fatalf("workload %q not found in %+v", name, app.Workloads)
	return v1.Workload{}
}

func envValue(w v1.Workload, name string) string {
	for _, ev := range w.Run.Env {
		if ev.Name == name {
			return ev.Value
		}
	}
	return ""
}
