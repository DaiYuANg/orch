name        = "my-app"
description = "Sample workload for testing"
include     = ["common.hcl", "test.hcl"]
datacenters = ["dc1", "dc2"]

resources {
  cpu    = 0
  memory = 0
  network {
    mbits = 0
  }
}

units {
  name = "backend"

  resources {
    cpu    = 2000
    memory = 2048
    network {
      mbits = 0
    }
  }

  tasks {
    name     = "api"
    type     = "service"
    driver   = "docker"
    image    = "ghcr.io/example/api:latest"
    replicas = 2

    env = {
      PORT = "8080"
    }

    network {
      name = "api-net"
      port = {
        http = 8080
      }
    }

    check {
      type     = "http"
      path     = "/health"
      interval = "10s"
      retries  = 3
      timeout  = "3s"
    }

    resources {
      cpu    = 1000
      memory = 1024
      network {
        mbits = 0
      }
    }
  }
}
