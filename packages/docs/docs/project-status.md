---
sidebar_position: 2
title: Warden 技术文档
description: 面向用户与运维的使用指南、整体架构与关键技术细节。
---

# Warden 技术文档

本文档基于当前仓库实现整理，目标是帮助你快速理解和使用 Warden，并掌握关键架构与技术细节。

## 1. Warden 是什么

Warden 是一个面向长生命周期服务（数据库、消息队列、对象存储等）的轻量运行时与控制平面，核心能力包括：

- 多运行时执行抽象（docker/containerd/systemd/firecracker/windows-service）
- 部署、调度、迁移、故障转移、重平衡
- Registry + Ingress + DNS 一体化服务发现与流量入口
- 可选 Raft 共识写入与多节点控制
- `cmd/server` 与 `cmd/cli` 分进程架构

## 2. 快速开始

### 2.1 前置依赖

- Go
- Docker（若使用 docker driver）
- Node.js 20+ 与 pnpm（若使用 dashboard/docs）

### 2.2 启动服务端

```bash
go run ./cmd/server run
```

默认 HTTP API 监听 `http://127.0.0.1:7443`。
默认同时监听本地 UDS：`$TMPDIR/warden.sock`（CLI 默认使用）。

### 2.3 获取访问 Token

```bash
go run ./cmd/cli token
```

默认 token 文件位置：

- Windows: `%TEMP%/warden.token`
- Linux/macOS: `/tmp/warden.token`

### 2.4 部署示例 workload

```bash
# CLI 默认走 UDS，无需显式 --api
go run ./cmd/cli service deploy --file ./examples/minimal-nginx.yaml
go run ./cmd/cli service list
go run ./cmd/cli service get <deployment-id>

# 如需强制走 HTTP（远程或调试场景）
go run ./cmd/cli --api http://127.0.0.1:7443 service list
```

### 2.5 验证 Ingress

```bash
curl -H "Host: demo.warden.local" http://127.0.0.1:8088/
```

### 2.6 查看日志与停止部署

```bash
go run ./cmd/cli --api http://127.0.0.1:7443 service logs <instance-id> --tail 200
go run ./cmd/cli --api http://127.0.0.1:7443 service stop <deployment-id>
```

## 3. 整体架构

### 3.1 组件分层

1. API 层（Fiber + Huma）
2. 编排层（task service）
3. 运行时层（runtime executors）
4. 数据层（registry + raft + dns store）
5. 数据面（ingress + dns）

### 3.2 关键模块

- `cmd/server`: 控制平面进程入口
- `cmd/cli`: 用户操作入口（deploy/migrate/failover/rebalance 等）
- `internal/task`: 部署编排、健康检查、重启恢复、迁移策略
- `internal/runtime_engine`: 各 runtime 适配实现
- `internal/registry`: endpoints/routes 持久化与路由解析
- `internal/ingress`: HTTP/TCP/UDP 入口代理
- `internal/dns`: DNS 记录持久化与解析（含热点缓存）
- `internal/raft`: 共识写入、FSM、snapshot/restore、缓存

### 3.3 核心数据流

1. 用户通过 CLI/HTTP 提交 DSL 到 `/tasks/deploy`
2. task 服务解析 DSL，生成实例并调度目标节点
3. runtime 执行实例启动，返回 container/unit/vm 标识
4. task 将 endpoint/route 注册到 registry
5. ingress 与 dns 基于 registry 提供流量接入与解析
6. reconcile loop 周期探测状态并执行重启/修复

## 4. Runtime 抽象与驱动

当前支持的 driver：

- `docker`
- `containerd`
- `systemd`（Linux）
- `firecracker`（Linux）
- `windows-service`（Windows）

说明：

- docker 路径最完整，含健康检查、重启、恢复。
- containerd 为 baseline，日志与部分语义仍在补齐。
- systemd/firecracker/windows-service 已接入 task runtime 工厂，但生产级语义仍在完善。

## 5. DSL 与部署模型

### 5.1 基本结构

- `workload` 下包含多个 `unit`
- `unit` 下包含多个 `task`
- 每个 task 可设置 `driver`、`replicas`、`check`、`labels`、`network`、`dns`

### 5.2 示例（docker 服务）

```yaml
name: demo-nginx
units:
  - name: web
    tasks:
      - name: nginx
        type: service
        driver: docker
        image: nginx:stable-alpine
        replicas: 1
        labels:
          warden.ingress.http.enable: "true"
          warden.ingress.http.host: "demo.warden.local"
          warden.ingress.http.path: "/"
          warden.ingress.http.port: "8080"
        network:
          port:
            http: 8080
        check:
          type: http
          path: /
          interval: 10s
          timeout: 3s
          retries: 3
```

### 5.3 Stateful 任务基线策略

当 task 设置 `stateful: true` 时：

- migrate/failover/rebalance 默认拒绝直接迁移
- 必须显式传 `force_stateful=true`
- 当前只允许 `max_unavailable=1`

CLI 示例：

```bash
go run ./cmd/cli service migrate <deployment-id> --target-node <node-id> --force-stateful --max-unavailable 1
```

## 6. 核心 API 与 CLI

### 6.1 常用 API

- `POST /tasks/deploy`
- `GET /tasks`
- `GET /tasks/{id}`
- `POST /tasks/{id}/stop`
- `GET /tasks/instances/{id}/logs`
- `POST /tasks/{id}/migrate`
- `POST /tasks/failover`
- `POST /tasks/rebalance`
- `GET /system/info`
- `GET /system/cluster`
- `POST /system/cluster/join`
- `POST /system/cluster/remove`

### 6.2 常用 CLI

```bash
go run ./cmd/cli service list
go run ./cmd/cli service deploy --file ./examples/minimal-nginx.yaml
go run ./cmd/cli service migrate <deployment-id> --target-node <node-id>
go run ./cmd/cli service failover --failed-node <node-id>
go run ./cmd/cli service rebalance --max-migrations 5
go run ./cmd/cli cluster status
go run ./cmd/cli info
```

CLI 通讯端点：

- 默认：`unix://$TMPDIR/warden.sock`
- 可切换：`--api http://127.0.0.1:7443`

## 7. 多节点与 Raft

本仓库提供本地多节点样例配置：`examples/local-raft/node{1..4}.yaml`。

典型流程：

1. 分别启动多个 server 进程（不同 `--conf`）
2. 在 leader 节点执行部署
3. 通过 `cluster status` 验证集群成员
4. 通过 `migrate/failover/rebalance` 验证调度行为

可使用内置 smoke：

```bash
task raft:smoke
```

## 8. 认证与持久化

### 8.1 认证

- API 受 JWT 中间件保护
- server 启动时生成 root token（默认有效期 72 小时）
- 默认 token 文件：`<temp>/warden.token`
- JWT 私钥默认持久化在：`$XDG_DATA_HOME/warden/auth/jwt_private_key.pem`
- 可通过环境变量 `WARDEN_AUTH_PRIVATE_KEY_FILE` 指定私钥文件

### 8.2 持久化存储

- Registry（routes/endpoints）：`$XDG_DATA_HOME/warden/registry.db`
- DNS records：`$XDG_DATA_HOME/warden/dns.db`
- Raft data（启用时）：`raft.data_dir`（默认 `$XDG_DATA_HOME/warden`）

## 9. 默认端口与配置要点

默认配置（可通过 `--conf` 覆盖）：

- HTTP API: `7443`
- DNS: `:1053`
- Ingress HTTP: `:8088`
- Raft bind: `127.0.0.1:12000`（默认关闭）

常见配置项：

- `http.port`
- `network.dns_listen`
- `network.ingress_http_listen`
- `raft.enable`
- `raft.node_id`
- `raft.bind_addr`
- `raft.api_addr`
- `raft.node_api`
- `raft.data_dir`

## 10. 前端与文档工作区

当前前端包已纳入 pnpm workspace：

- `packages/dashboard`（Refine + shadcn 风格）
- `packages/docs`（Docusaurus）

Dashboard 当前定位为只读观测入口，主要页面：

- Deployments：已部署 workload 与实例状态
- Network：registry routes 与 endpoints（网络流量映射）
- DNS：DNS 持久化记录
- System：主机资源与节点信息

常用命令：

```bash
pnpm -C packages/docs start
pnpm -C packages/docs build
pnpm -C packages/dashboard dev
```

## 11. 当前限制与建议

当前仍建议将 docker 路径作为生产主路径。非 docker runtime 与 stateful 编排策略正在持续增强，重点方向包括：

- containerd/systemd/firecracker/windows-service 的语义对齐
- stateful 场景下 drain、pre-check、rollback、更细粒度可用性预算
- dashboard 的部署与运维闭环完善
