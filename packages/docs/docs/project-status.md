---
sidebar_position: 2
title: 项目状态与使用指南
description: 基于当前 Rust 代码的可用能力、接口、运行方式与限制说明。
---

# 项目状态与使用指南

本文档对应当前仓库实现（Rust workspace），用于快速上手与确认可用能力。

## 1. 当前实现范围

- 后端已迁移为 Rust crate 架构（`apps/*` + `crates/*`）。
- API 统一返回 `ApiEnvelope`，业务错误码稳定化：
  - `0` 成功
  - `1001` 参数错误
  - `1004` 未找到
  - `2000` 内部错误
- API 层已接入 OpenAPI + Swagger UI（`utoipa`）。
- CLI 已支持平台优先通讯：`unix://` / `npipe://` 失败后 fallback `http://`。
- Ingress / DNS 有基础能力，支持热点内存缓存（`moka`）与持久化存储（`redb`/`memory`）。

## 2. 快速启动

## 2.1 依赖

- Rust stable
- Docker（需要 `docker` runtime 时）
- Node.js 20+ 与 pnpm（文档与 dashboard）

## 2.2 启动服务

```bash
just run
```

或

```bash
cargo run -p warden-server-rs -- --conf examples/config/local.yaml
```

## 2.3 CLI 调用

```bash
cargo run -p warden-cli-rs -- --api auto workloads
cargo run -p warden-cli-rs -- --api auto deploy --name demo --runtime docker --host demo.local --port 8080 --ingress-port 18088
cargo run -p warden-cli-rs -- --api auto task list
```

`--api auto` 优先使用本机协议（Unix Domain Socket / Named Pipe），失败后自动回退 HTTP。

## 3. API 与文档入口

- API 基地址：`http://127.0.0.1:7443`
- OpenAPI JSON：`/api-docs/openapi.json`
- Swagger UI：`/swagger-ui/`

常用接口：

- `GET /healthz`
- `GET /workloads`
- `GET /tasks`
- `GET /tasks/{id}`
- `POST /tasks/deploy`
- `POST /tasks/{id}/stop`
- `GET /system/endpoints`
- `GET /system/routes`
- `GET /system/dns/records`

## 4. 关键技术细节

- 配置加载：`figment`（默认值 + 文件 + `WARDEN__...` 环境变量覆盖）。
- 配置校验：`validator`（含 timeout 合法性与 store schema 校验）。
- HTTP client 重试：`reqwest-middleware + reqwest-retry`，仅对幂等方法启用重试。
- DNS server：`hickory-dns`。
- Raft：`openraft`（集群能力处于持续完善阶段）。
- 构建编排：`just` + `cargo xtask`（替代 Taskfile）。

## 5. 当前限制

- Runtime 以 `docker` 路径最完整；`containerd/firecracker/...` 仍在补齐。
- migrate/failover/rebalance 仍在完善，暂不建议作为生产级编排闭环依赖。
- dashboard 目前偏观测定位，写操作建议使用 CLI/API。
