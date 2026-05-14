# orch Ingress Design v1

> This repository is **orch**. Older prose may say “Warden” for the same control-plane lineage; configuration uses the **`ORCH`** environment prefix.

Status: Server-side design specification for the built-in ingress system.

Chinese version: `ingress.zh.md`

This document defines the server-side ingress direction for Warden.
It focuses on the built-in ingress runtime and reverse-proxy model rather than
the DSL surface alone.

## Goals

Warden ingress should be built in, not delegated to an external stack that
users need to assemble themselves.

The v1 ingress system should provide:

- A built-in HTTP/HTTPS ingress runtime
- Reverse proxy routing from public traffic to workload endpoints
- Cluster-wide route distribution without an external ingress controller
- A simple workload publishing model based on domain and path routing
- Health-aware backend selection using Warden-native endpoint state

The design should avoid the Kubernetes-style outcome where a user must still
choose and integrate:

- An ingress controller
- A reverse proxy implementation
- A service discovery layer
- An external route distribution scheme

## Core Model

Ingress in Warden should have two layers:

1. A control-plane object: `ingress`
2. A built-in data-plane service: `ingress runtime + reverse proxy`

This means:

- DSL and API define ingress intent
- Warden stores and distributes that intent
- Every ingress-capable node runs a built-in ingress runtime
- The ingress runtime resolves healthy workload endpoints and proxies traffic to them

## Architectural Direction

The recommended architecture is:

```text
DSL / API
  -> canonical ingress model
  -> store / raft
  -> ingress route snapshot
  -> per-node ingress runtime
  -> reverse proxy to healthy workload endpoints
```

The ingress runtime is the embedded `internal/ingress` subsystem backed by
`github.com/arcgolabs/vale`; `orch-server` remains only the composition root
that wires and starts it.

Current implementation status as of May 8, 2026:

- ingress data-plane routing uses `github.com/arcgolabs/vale v0.1.3`
  runtime/proxy directly
- the runtime consumes compiled route snapshots built from desired deploy apps
  and DNS workload records
- runtime route records now also carry explicit backend binding fields for
  workload and endpoint identity, so snapshot compilation can prefer explicit
  endpoint binding over backend-address inference
- compiled runtime snapshots now also keep explicit backend binding identity on
  HTTP and stream routes, rather than reducing everything to backend address
  strings only
- internally, ingress now distinguishes control-plane route objects from
  runtime snapshot routes, instead of treating the stored route record and the
  runtime route shape as the same thing
- route snapshot compilation already prefers endpoint-backed candidates filtered
  by endpoint `healthy` and `ready` state before falling back to raw backend
  strings

## Control Plane

The canonical control-plane object remains top-level `ingress`.

Example authoring shape:

```kotlin
ingress("public") {
  host("api.example.com")
  route("/") {
    backend(workloads.api.endpoint("http"))
  }
}
```

The canonical ingress object should at minimum cover:

- `name`
- `host`
- `routes[]`
- `tls`
- `entrypoints`
- `policy`

Each route should at minimum cover:

- `path`
- `backend: EndpointRef`
- `rewrite`
- `timeout`
- `headers`

v1 only needs a small subset of those fields, but the object boundary should be
frozen now.

## Data Plane

Each ingress-enabled node should run a built-in ingress runtime.

That runtime should:

- Listen on configured public addresses
- Hold a read-optimized route snapshot
- Resolve backend endpoints from registry state
- Select healthy backends
- Reverse proxy traffic to the selected backend

The runtime should not invent its own configuration source. It should consume a
compiled ingress route table generated from the canonical model and stored in
cluster state.

## Request Flow

Recommended request path:

```text
client
  -> node ingress runtime
  -> host/path match
  -> backend endpoint resolution
  -> healthy backend selection
  -> reverse proxy to workload endpoint
```

The ingress runtime should use Warden-native endpoint records rather than raw static
addresses as its source of truth.

## Endpoint and Backend Model

Backends should always be modeled as `EndpointRef`, not arbitrary socket
strings.

Preferred:

```kotlin
backend(workloads.api.endpoint("http"))
```

Not preferred:

```kotlin
backend("10.0.0.5:8080")
```

This keeps ingress tied to the canonical workload graph and lets the ingress runtime
reuse health, readiness, and placement information.

## Workload-Local Publishing Sugar

The canonical model should remain top-level `ingress`, but authoring may expose
workload-local sugar later.

Possible future sugar:

```kotlin
workload("api") {
  endpoint("http") {
    port(8080)
    protocol(http)
  }

  publish("public") {
    host("api.example.com")
    path("/")
    endpoint("http")
  }
}
```

That should lower to:

```kotlin
ingress("public") {
  host("api.example.com")
  route("/") {
    backend(workloads.api.endpoint("http"))
  }
}
```

The sugar is optional. The canonical control-plane object remains `ingress`.

## Ingress Runtime Responsibilities

The built-in ingress subsystem should eventually be split into these
responsibilities:

1. `RouteManager`
   Compiles canonical ingress objects into a runtime route table

2. `BackendResolver`
   Resolves `EndpointRef` into healthy backend candidates

3. `IngressRuntime`
   Owns listeners, route matching, and request dispatch

4. `ReverseProxy`
   Handles HTTP forwarding, headers, timeouts, websocket, and streaming

## Registry and State Requirements

Ingress should depend on explicit endpoint state instead of implicit inference.

The registry-facing endpoint record should be able to provide:

- workload id or workload name
- node id
- endpoint name
- protocol
- target address
- target port
- healthy
- ready
- last update time

The ingress runtime should only route to endpoints that satisfy ingress eligibility for
the selected protocol and health policy.

## Node Topology

The default HA model should be simple:

- Every ingress-enabled node runs an ingress runtime
- Route configuration is distributed through raft/store state
- External DNS may point to one or multiple nodes
- Each node can proxy to healthy backends across the cluster

This avoids requiring a separate dedicated ingress tier in v1.

Dedicated ingress node roles, VIPs, BGP, or edge-only placement may be added
later, but should not be a prerequisite for the first built-in ingress release.

## HTTP Behavior

The first implementation should focus on HTTP/HTTPS only.

v1 behaviors to support:

- host-based routing
- path-based routing
- reverse proxy forwarding
- `Host` and `X-Forwarded-*` handling
- websocket pass-through
- request timeout configuration
- simple round-robin load balancing
- health-aware backend filtering

The first implementation should not block on:

- ACME automation
- advanced auth filters
- rate limiting
- canary and traffic splitting
- WAF behavior
- full TCP/UDP ingress parity

## TLS

TLS should be part of the ingress object, but the first implementation can stay
minimal.

Recommended v1 TLS support:

- static certificate/key reference
- secret- or file-backed source
- per-host TLS binding

ACME and automatic certificate lifecycle can come later.

## Canonical Ingress Schema

Recommended target shape:

```text
Ingress
- name
- host
- entrypoints[]
- tls
  - enabled
  - certificate_ref
  - private_key_ref
- routes[]
  - path
  - backend: EndpointRef
  - rewrite
  - timeout
  - headers
  - policy
```

v1 can implement a smaller active subset:

```text
Ingress
- name
- host
- routes[]
  - path
  - backend: EndpointRef
```

## Server Composition Guidance

At the server layer, ingress should remain an explicitly wired subsystem in the
main composition root.

Recommended relationship:

```text
orch-server
  -> ingress service
  -> registry service
  -> store / raft
  -> runtime / task state
```

`orch-server` should not own ingress routing logic. It should only construct,
wire, and start the ingress subsystem.

The ingress service should not directly own workload lifecycle. It should
consume registry and route snapshots generated by the rest of the system.

## Package Boundary Direction

The ingress boundary should keep data-plane behavior in `arcgolabs/vale` and
orch-specific control-plane translation in `internal/ingress`.

Recommended package layout:

- `internal/ingress`
  Orch adapter: compile deploy/DNS state, configure listeners, refresh snapshots.
- `github.com/arcgolabs/vale/runtime`
  Runtime route table, gateway, entrypoint, service, endpoint, and matching.
- `github.com/arcgolabs/vale/proxy`
  Reverse proxy data plane.
- future stream package
  Later TCP/UDP ingress if and when stream ingress becomes a real productized
  path.

Responsibility boundary:

- `orch-server`
  composition root only
- `arcgolabs/vale`
  ingress control-plane consumption and data-plane runtime

## Relationship to `arcgolabs/vale`

`arcgolabs/vale` is the ingress runtime/proxy dependency for this repository.
`internal/ingress` owns only orch-specific integration:

- compiling deploy/DNS state into vale route snapshots
- starting HTTP/HTTPS listeners
- wiring TLS/autocert configuration
- refreshing snapshots when desired deploy state changes

The design direction should keep vale as the data-plane boundary:

- keep HTTP ingress runtime and reverse proxy as the main v1 focus
- keep route compilation and backend selection in explicit orch layers
- treat TCP/UDP stream ingress as later expansion, not the primary v1 path

## Rollout Plan

Recommended implementation order:

1. Freeze this ingress design
2. Add canonical ingress schema and route table types
3. Keep `internal/ingress` as the orch adapter over `arcgolabs/vale`
4. Wire route snapshot loading from server state
5. Implement HTTP host/path routing against healthy endpoint records
6. Add basic TLS handling

## Explicit Non-Goals for v1

The first built-in ingress release should not try to solve every edge problem.

Non-goals:

- full L7 traffic policy system
- service mesh replacement
- plugin runtime
- automatic certificate management from day one
- mandatory dedicated ingress node topology
- forcing users to deploy an external ingress product

## Summary

The ingress direction for Warden should be:

- top-level canonical `ingress` object
- built-in per-node ingress runtime
- reverse proxy data plane
- backend routing strictly through `EndpointRef`
- HTTP/HTTPS first
- cluster-distributed route snapshots
- health-aware backend selection

That gives Warden a built-in publishing model for workloads without pushing
network architecture selection back onto users.
