---
sidebar_position: 3
title: Rust Rewrite 状态
---

# Rust Rewrite 状态

本文档描述 Go `internal/*` 到 Rust `crates/*` 的一比一迁移结构，以及当前已完成的能力。

## 1. 模块映射

| Go 模块 | Rust crate | 说明 |
| --- | --- | --- |
| `internal/config` | `crates/warden-config` | 配置模型与加载（`figment`: 默认值 + 文件 + 环境变量覆盖） |
| `internal/logger` | `crates/warden-logger` | `tracing` 初始化 |
| `internal/http` | `crates/warden-http` | HTTP 监听 + UDS 配置位 |
| `internal/endpoint` | `crates/warden-api` | Axum 路由与系统/工作负载只读接口 |
| `internal/registry` | `crates/warden-registry` | workload/route/endpoint 查询服务 |
| `internal/*` 持久化 | `crates/warden-store` | KV 抽象层（`memory`/`redb` 可替换后端） |
| `internal/dns` | `crates/warden-dns` | DNS 记录查询服务 |
| `internal/ingress` | `crates/warden-ingress` | ingress 服务骨架 |
| `internal/task` | `crates/warden-task` | task 编排服务骨架 |
| `internal/runtime_engine` | `crates/warden-runtime` | runtime 抽象骨架 |
| `internal/raft` | `crates/warden-raft` | raft 服务骨架 |
| `internal/model` | `crates/warden-types` | API 与领域 DTO |
| `cmd/cli` API 通讯层 | `crates/warden-client` | HTTP/UDS endpoint 解析与请求 |
| `cmd/server` | `apps/warden-server-rs` | Rust 控制面入口 |
| `cmd/cli` | `apps/warden-cli-rs` | Rust 用户入口 |

## 2. 已完成能力

- Rust workspace 与 crate 拆分已建立。
- `warden-server-rs` 可启动并暴露只读接口：
  - `GET /healthz`
  - `GET /workloads`
  - `GET /system/endpoints`
  - `GET /system/routes`
  - `GET /system/dns/records`
- 基础写路径已接入（非 DSL）：
  - `POST /tasks/deploy`
  - `GET /tasks`
  - `GET /tasks/{id}`
  - `POST /tasks/{id}/stop`
- `warden-cli-rs` 可查询：`workloads/endpoints/routes/dns`。
- `warden-cli-rs` 已支持 `deploy/stop/task list/task get`（JSON 参数，不依赖旧 HCL DSL）。
- `warden-cli-rs` 支持 `--api auto`：优先平台协议（Unix/Named Pipe）失败后 fallback 到 HTTP。
- 构建与编排入口切换为 `just + cargo xtask`。
- DNS server 底座改为 `hickory-dns`（`hickory-server`）。
- 配置系统切换为 `figment`，支持 `WARDEN__...` 环境变量覆盖。
- KV 存储增加抽象层：默认 `redb`，可切换 `memory`。

## 3. 待完成（达到 Go 全量替代前）

- migrate/failover/rebalance 写路径。
- runtime driver 具体实现：docker/containerd/systemd/firecracker/windows-service。
- raft 一致性写入与恢复流程。
- ingress TCP/UDP 动态路由与健康探测。
- DNS 热点缓存淘汰策略与存储同步策略。
- 认证与 token 签发（JWT）链路。

## 4. 本地运行

```bash
just check
just run
cargo run -p warden-cli-rs -- --api auto workloads
cargo run -p warden-cli-rs -- --api auto deploy --name demo-api --runtime docker --host demo.local --port 8080 --ingress-port 18088
cargo run -p warden-cli-rs -- --api auto task list
just cluster-up
just cluster-status
just cluster-down
```

## 5. 关于删除原 Go 源码

建议在以下条件全部满足后删除 Go 源码：

1. Rust CLI 覆盖全部当前 CLI 子命令。
2. Dashboard 依赖的全部接口在 Rust 侧完成。
3. 本地多节点 Raft smoke（含 failover/rebalance）在 Rust 通过。
4. 至少一次灰度环境回归通过。

当前仍处于“架构落地 + 只读路径可用”阶段，不建议立即删除 Go 代码。
