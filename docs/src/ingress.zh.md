# Warden Ingress 设计 v1

状态：Warden 内建 ingress 系统的 server 侧设计规范。

对应英文版：`ingress.md`

本文档定义 Warden ingress 的 server 侧方向，重点是内建 ingress runtime 与
reverse proxy 的架构，而不只是 DSL 写法。

## 目标

Warden 的 ingress 应当是内建能力，而不是把网络入口方案继续丢给用户选型。

v1 ingress 系统应提供：

- 内建 HTTP/HTTPS ingress runtime
- 从公网流量到 workload endpoint 的反向代理
- 不依赖外部 ingress controller 的集群级路由分发
- 基于域名与路径的简单 workload 发布模型
- 结合 Warden 原生 endpoint 状态的健康后端选择

这个方向要避免 Kubernetes 常见的结果：

- 用户还得自己选 ingress controller
- 自己选 reverse proxy
- 自己拼 service discovery
- 自己处理 route 分发

## 核心模型

Warden ingress 应拆成两层：

1. 控制面对象：`ingress`
2. 数据面服务：内建 `ingress runtime + reverse proxy`

也就是说：

- DSL / API 表达 ingress 意图
- Warden 存储并分发这份意图
- 每个启用 ingress 的节点运行一个内建 ingress runtime
- ingress runtime 解析健康 workload endpoint，并把流量代理过去

## 架构方向

推荐结构：

```text
DSL / API
  -> canonical ingress model
  -> store / raft
  -> ingress route snapshot
  -> per-node ingress runtime
  -> reverse proxy to healthy workload endpoints
```

ingress runtime 属于 `warden-ingress` 子系统，`warden-server` 只负责装配、
启动与编排，不承载 ingress 路由逻辑本身。

截至 2026 年 3 月 26 日的当前实现状态：

- `warden-ingress` 已经拆分为 `warden-ingress`、`warden-ingress-types`、
  `warden-ingress-http`、`warden-ingress-proxy`、`warden-ingress-resolver`
  与 `warden-ingress-stream`
- runtime 已经消费编译后的 `IngressRouteSnapshot`，而不是在请求路径上直接
  扫原始 `RouteRecord`
- runtime route record 现在也开始携带显式的 workload / endpoint 绑定字段，
  因此 snapshot 编译会优先使用显式 endpoint binding，而不是先靠 backend
  地址字符串反推
- 编译后的 runtime snapshot 现在也会在 HTTP / stream route 上保留显式的
  backend binding 身份，而不再把全部语义都降成 backend 地址字符串
- 在内部实现上，ingress 现在也已经把 control-plane route object 与
  runtime snapshot route 正式区分开来，而不再把 store 里的 route record
  和 runtime route shape 当成同一个东西
- route snapshot 编译已经会优先选择由 endpoint state 支撑、并经过
  `healthy/ready` 过滤的 backend；只有兜底时才回退到原始 backend 字符串

## 控制面

canonical 控制面对象仍然保持顶层 `ingress`。

示例：

```kotlin
ingress("public") {
  host("api.example.com")
  route("/") {
    backend(workloads.api.endpoint("http"))
  }
}
```

canonical ingress 对象建议至少覆盖：

- `name`
- `host`
- `routes[]`
- `tls`
- `entrypoints`
- `policy`

每个 route 至少覆盖：

- `path`
- `backend: EndpointRef`
- `rewrite`
- `timeout`
- `headers`

v1 实现可以先做更小子集，但对象边界应先冻结。

## 数据面

每个 ingress-enabled node 都应运行一个内建 ingress runtime。

它负责：

- 监听配置好的公网地址
- 持有只读优化的 route snapshot
- 从 registry 解析 backend endpoint
- 选择健康后端
- 做反向代理转发

它不应再自造配置源，而应消费由 canonical model 编译出来并分发到集群状态中的
route table。

## 请求路径

推荐请求路径：

```text
client
  -> node ingress runtime
  -> host/path match
  -> backend endpoint resolution
  -> healthy backend selection
  -> reverse proxy to workload endpoint
```

ingress runtime 应以 Warden-native endpoint record 为事实来源，而不是静态
socket 字符串。

## Endpoint 与 Backend 模型

backend 永远应表达为 `EndpointRef`，而不是任意地址字符串。

推荐：

```kotlin
backend(workloads.api.endpoint("http"))
```

不推荐：

```kotlin
backend("10.0.0.5:8080")
```

这样 ingress 就能和 canonical workload graph、健康状态、调度状态保持一致。

## Workload 侧发布糖语法

canonical model 仍建议保留顶层 `ingress`，但 authoring 层未来可以提供
workload-local sugar。

例如：

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

再 lower 为：

```kotlin
ingress("public") {
  host("api.example.com")
  route("/") {
    backend(workloads.api.endpoint("http"))
  }
}
```

糖语法是可选的，canonical 控制面对象仍是 `ingress`。

## Ingress Runtime 责任划分

内建 ingress 子系统建议逐步收敛成以下职责：

1. `RouteManager`
   把 canonical ingress 编译成运行时 route table

2. `BackendResolver`
   把 `EndpointRef` 解析成健康 backend 列表

3. `IngressRuntime`
   持有 listener、路由匹配与请求分发逻辑

4. `ReverseProxy`
   处理 HTTP 转发、header、timeout、websocket、streaming

## Registry 与状态要求

Ingress 应依赖显式 endpoint 状态，而不是隐式猜测。

面向 ingress runtime 的 endpoint record 至少应能提供：

- workload id 或 workload name
- node id
- endpoint name
- protocol
- target address
- target port
- healthy
- ready
- last update time

ingress runtime 只路由到满足协议与健康策略要求的 endpoint。

## 节点拓扑

默认 HA 模型应尽量简单：

- 每个 ingress-enabled node 都运行 ingress runtime
- route 配置通过 raft/store 分发
- 外部 DNS 可指向一个或多个 node
- 每个 node 都可以代理到集群内健康 backend

这样 v1 不需要单独再做 dedicated ingress tier。

专门的 ingress node role、VIP、BGP、edge-only placement 都可以后加，但不应成为
第一版内建 ingress 的前置条件。

## HTTP 行为

第一版先聚焦 HTTP/HTTPS。

v1 需要支持：

- host 路由
- path 路由
- reverse proxy 转发
- `Host` 与 `X-Forwarded-*` 处理
- websocket 透传
- 请求超时配置
- 简单 round-robin 负载均衡
- 基于健康状态的 backend 过滤

v1 不应阻塞在这些高级能力上：

- ACME 自动签证
- 高级认证过滤器
- rate limiting
- canary / traffic splitting
- WAF
- 完整 TCP/UDP ingress 对等能力

## TLS

TLS 应属于 ingress 对象，但第一版可以先做最小闭环。

推荐 v1 TLS 支持：

- 静态证书/私钥引用
- secret 或 file 作为来源
- 按 host 绑定 TLS

ACME 与自动证书生命周期后续再加。

## Canonical Ingress Schema

推荐目标结构：

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

v1 实际可以先做更小 active subset：

```text
Ingress
- name
- host
- routes[]
  - path
  - backend: EndpointRef
```

## Server 组合关系

在 server 层，ingress 应继续作为一个显式 wiring 的 subsystem 存在于主组合根。

推荐关系：

```text
warden-server
  -> ingress service
  -> registry service
  -> store / raft
  -> runtime / task state
```

`warden-server` 不应拥有 ingress 路由逻辑；它只负责构建、装配并启动
ingress 子系统。

ingress service 不应自己管理 workload 生命周期，它应消费 registry 和 route
snapshot。

## Crate 拆分方向

ingress 这一块目前可以直接一步到位拆到目标结构，因为现有
`warden-ingress` 体量还不大，而且边界已经比较清楚。

建议的 crate 结构：

- `warden-ingress`
  ingress 子系统 facade 与 service 入口
- `warden-ingress-types`
  route snapshot、backend 引用、listener 配置、runtime-facing 类型
- `warden-ingress-http`
  HTTP/HTTPS 路由匹配与请求分发
- `warden-ingress-proxy`
  reverse proxy、header 转发、websocket、streaming、timeout 处理
- `warden-ingress-resolver`
  基于 registry 与 endpoint 健康状态的 backend 解析
- `warden-ingress-stream`
  未来如需把 TCP/UDP ingress 做成正式产品路径，再单独承载 stream ingress

职责边界：

- `warden-server`
  仅作为 composition root
- `warden-ingress-*`
  承担 ingress 控制面消费与数据面 runtime

## 与当前 `warden-ingress` crate 的关系

当前 crate 已经有：

- HTTP proxy 路径
- TCP/UDP stream proxy helper
- ingress service bootstrap

后续应把它收敛成更清晰的结构：

- 保持 HTTP ingress runtime + reverse proxy 作为 v1 主线
- 把 route compilation 与 backend 选择提升为显式层次
- TCP/UDP stream ingress 放到后续扩展，而不是第一版主路径

也就是说，这个 crate 应逐步演进成：

- 控制面 route snapshot 消费者
- per-node ingress runtime
- reverse proxy 实现

而不是继续停留在一组松散的 proxy helper 上。

## 落地顺序

推荐实现顺序：

1. 冻结这份 ingress 设计
2. 增加 canonical ingress schema 与 route table 类型
3. 将 ingress 直接拆分为 `warden-ingress`、`warden-ingress-types`、
   `warden-ingress-http`、`warden-ingress-proxy`、`warden-ingress-resolver`
4. 从 server 状态中加载 route snapshot
5. 实现 HTTP host/path 路由与健康 endpoint 反向代理
6. 增加基础 TLS

## v1 明确非目标

第一版内建 ingress 不需要一次性解决所有边缘能力。

非目标：

- 完整 L7 流量策略系统
- service mesh 替代品
- 插件运行时
- 一开始就做自动证书管理
- 强制 dedicated ingress node 拓扑
- 依赖外部 ingress 产品才能工作

## 总结

Warden 的 ingress 方向应当是：

- 顶层 canonical `ingress` 对象
- 内建 per-node ingress runtime
- reverse proxy 数据面
- backend 严格通过 `EndpointRef`
- 先做 HTTP/HTTPS
- 集群级 route snapshot 分发
- 健康后端选择

这样 Warden 才能提供真正内建的 workload 发布能力，而不是把网络入口的架构选型再次推回给用户。
