# Quick Start

## Prerequisites

- Go 1.22+
- Docker (for docker runtime)
- containerd (optional, for containerd runtime later)

## Build and test

```bash
go mod tidy
go test ./...
```

## Run CLI (parse deploy YAML)

```bash
go run ./cmd/orch-cli dsl parse --file path/to/app.yaml --json
```

## Run server

```bash
go run ./cmd/orch-server
```
