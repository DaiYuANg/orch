package orch_test

import (
	"testing"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func TestOrchShortFormSugar(t *testing.T) {
	app := loadAppString(t, "short.orch", `app {
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
}`)
	requireMetadata(t, app, "short", "demo")
	requireShortFormDB(t, workloadByName(t, app, "db"))
	requireShortFormAPI(t, workloadByName(t, app, "api"))
	requireIngressRoute(t, app, 0, "/", "api", "api")
	requireValidApp(t, app)
}

func requireShortFormDB(t *testing.T, db v1.Workload) {
	t.Helper()
	requireKindRuntime(t, db, v1.WorkloadKindStateful, v1.RuntimeDocker)
	requireStatefulScheduling(t, db)
	requireDockerNetwork(t, db, "orch-demo")
	requireResources(t, db, 1000, 1073741824)
}

func requireShortFormAPI(t *testing.T, api v1.Workload) {
	t.Helper()
	requireDependsOn(t, api, "db")
	requireEndpoint(t, api, "api", 8080, v1.ProtoHTTP)
	requireResources(t, api, 250, 268435456)
	requireEnv(t, api, "DATABASE_URL", "postgres://db/app")
}
