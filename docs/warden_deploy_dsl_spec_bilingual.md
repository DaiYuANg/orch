# Warden Deploy DSL Specification / Warden 部署 DSL 规范

> Version / 版本: Draft v0.1  
> Status / 状态: Working Draft  
> Scope / 范围: Deploy-oriented vertical DSL for Warden multi-runtime engine / 面向 Warden 多运行时引擎的部署垂直 DSL

---

# 1. Overview / 概述

## 1.1 Purpose / 目的

**EN**  
Warden Deploy DSL is a strongly-typed, deploy-oriented vertical domain-specific language designed for defining application deployment intent, lifecycle behavior, runtime bindings, service connectivity, and rollout-related configuration in a unified way across multiple runtimes.

It is not intended to be a generic infrastructure language, a general-purpose scripting language, or a templating layer over Kubernetes YAML. Instead, it is designed as a compile-time checked deploy language that targets Warden’s multi-runtime execution model.

**ZH**  
Warden Deploy DSL 是一门**强类型、面向部署场景的垂直领域 DSL**，用于统一描述应用的部署意图、生命周期行为、运行时绑定、服务连接关系以及发布相关配置，并面向多运行时后端进行编译。

它**不是**通用基础设施语言、不是通用脚本语言、也不是 Kubernetes YAML 的模板封装层。它的定位是：一门可在编译期完成校验的部署语言，并作为 Warden 多运行时执行模型的上层表达方式。

## 1.2 Design Goals / 设计目标

**EN**
1. Strong typing over deploy semantics.
2. Kotlin-DSL-like authoring experience with compile-time discipline.
3. Default-first design: common cases should require minimal syntax.
4. Explicit composition via profiles, capabilities, and reusable components.
5. Multi-runtime portability with backend lowering.
6. Explainability: every effective value should be traceable.
7. Static analyzability for CLI, LSP, planner, and runtime backends.
8. Controlled lifecycle flow support for init, checks, migration, and gating.

**ZH**
1. 以部署语义为中心的强类型设计。
2. 提供类似 Kotlin DSL / Gradle DSL 的编写体验，同时保持编译期约束。
3. 默认优先：常见场景下尽量少写。
4. 通过 profile、capability、可复用组件实现显式组合。
5. 面向多运行时后端的可移植性与 lowering 能力。
6. 可解释性：任一生效值都应可追踪来源。
7. 具备良好的静态分析能力，服务于 CLI、LSP、planner 与 backend。
8. 为 init、检查、迁移、门禁等场景提供受控生命周期流程能力。

## 1.3 Non-Goals / 非目标

**EN**
- General infrastructure provisioning like VPC, subnet, cloud IAM, or VM creation.
- Arbitrary user-defined scripting with unrestricted control flow.
- Text templating over platform-native manifests.
- Direct exposure of backend-native object models to end users.

**ZH**
- 不负责通用基础设施创建，例如 VPC、子网、云 IAM、虚拟机等。
- 不支持无限制的用户自定义脚本与任意控制流。
- 不做面向原生平台清单的文本模板系统。
- 不直接向终端用户暴露后端平台原生对象模型。

---

# 2. Language Positioning / 语言定位

## 2.1 Core Positioning / 核心定位

**EN**  
The DSL is an **Application Deploy DSL** and more specifically a **Multi-runtime Service Deploy DSL**. Its primary abstraction is the deployable workload rather than infrastructure resources.

**ZH**  
该 DSL 的定位是 **Application Deploy DSL（应用部署 DSL）**，更具体地说，是一门 **Multi-runtime Service Deploy DSL（多运行时服务部署 DSL）**。其核心抽象对象是“可部署工作负载”，而不是底层基础设施资源。

## 2.2 Core Abstractions / 核心抽象

**EN**
The language is centered around the following first-class deploy concepts:
- App
- Service
- Worker
- Job
- Profile
- Capability
- Run
- Resources
- Storage
- Expose
- Dependency
- Health
- Lifecycle
- Init Step
- Check

**ZH**
语言围绕以下一等部署概念构建：
- App
- Service
- Worker
- Job
- Profile
- Capability
- Run
- Resources
- Storage
- Expose
- Dependency
- Health
- Lifecycle
- Init Step
- Check

---

# 3. Syntax Style / 语法风格

## 3.1 Style Principles / 风格原则

**EN**
The syntax should be inspired by Kotlin DSL / Gradle DSL rather than HCL or YAML. The language should look like a statically analyzable declarative builder syntax, not like a free-form scripting language.

**ZH**
语法风格应借鉴 Kotlin DSL / Gradle DSL，而不是 HCL 或 YAML。整体风格应表现为**可静态分析的声明式 builder 语法**，而不是自由脚本语言。

## 3.2 Syntax Characteristics / 语法特征

**EN**
- Call-style declarations with scoped blocks.
- Assignment for typed fields.
- Nested typed scopes.
- Limited expressions.
- No indentation-sensitive structure.
- No text-template interpolation model as the main mechanism.

**ZH**
- 使用带作用域 block 的调用式声明。
- 通过赋值表达强类型字段。
- 支持嵌套的类型化作用域。
- 表达式能力受限。
- 不依赖缩进表达结构。
- 不以文本模板插值作为主要机制。

## 3.3 Canonical Example / 规范示例

```kotlin
import("./profiles/java.wd")
import("./caps/redis.wd")

app("order-system") {
  service("api") {
    use(profile("java-web")) {
      port = 8080
    }

    use(capability("redis-client"))

    run {
      app {
        image = image("registry.local/order-api:1.0.0")
        command = listOf("/app/server")
      }
    }

    resources {
      cpu = 500.milliCpu
      memory = 512.mebi
    }

    expose {
      publicHttp("api.example.com", target = 8080)
    }
  }
}
```

---

# 4. Type System / 类型系统

## 4.1 Type System Goals / 类型系统目标

**EN**
The DSL must be strongly typed in semantics, even if surface syntax stays concise. Compile-time validation must reject structurally invalid or semantically inconsistent deploy definitions.

**ZH**
DSL 必须在**语义层面保持强类型**，即使表层语法保持简洁。任何结构非法或语义不一致的部署定义，都应在编译期被拒绝。

## 4.2 Type Layers / 类型分层

### 4.2.1 Primitive and Value Types / 原始值与领域值类型

**EN**
The language should support at least:
- String
- Int
- Bool
- Duration
- Port
- Cpu
- Memory
- Size
- Host
- Path
- Url
- ImageRef

**ZH**
语言至少应支持以下类型：
- String
- Int
- Bool
- Duration
- Port
- Cpu
- Memory
- Size
- Host
- Path
- Url
- ImageRef

### 4.2.2 Structural Types / 结构类型

**EN**
Examples include:
- Resources
- HealthCheck
- ExposeRule
- StorageSpec
- EnvMap
- InitStep
- CheckExpr

**ZH**
结构类型示例包括：
- Resources
- HealthCheck
- ExposeRule
- StorageSpec
- EnvMap
- InitStep
- CheckExpr

### 4.2.3 Domain Types / 领域类型

**EN**
Examples include:
- ServiceDecl
- WorkerDecl
- JobDecl
- ProfileDecl
- CapabilityDecl
- RunSpec
- LifecycleStep

**ZH**
领域类型示例包括：
- ServiceDecl
- WorkerDecl
- JobDecl
- ProfileDecl
- CapabilityDecl
- RunSpec
- LifecycleStep

## 4.3 Typed Schemas / 类型化 Schema

**EN**
Each declaration and scope must have a typed schema. Only allowed members may appear within that scope.

Example constraints:
- `service` may define `run`, `resources`, `expose`, `dependsOn`, `lifecycle`.
- `worker` may not define public HTTP exposure.
- `job` may not define `replicas > 1` in v0.1.
- `run.app` must follow application runtime schema.

**ZH**
每个声明与作用域都必须拥有明确的类型化 Schema。该作用域内只能出现允许的成员。

约束示例：
- `service` 可以定义 `run`、`resources`、`expose`、`dependsOn`、`lifecycle`。
- `worker` 不允许定义公网 HTTP 暴露。
- `job` 在 v0.1 中不允许定义 `replicas > 1`。
- `run.app` 必须遵循应用运行单元的类型 Schema。

## 4.4 Typed Literals / 领域字面量

**EN**
The language should support domain-oriented literal forms such as:
- `30s`
- `500m`
- `512Mi`
- `10Gi`
- `8080`

**ZH**
语言应支持面向领域的字面量形式，例如：
- `30s`
- `500m`
- `512Mi`
- `10Gi`
- `8080`

---

# 5. Declaration Model / 声明模型

## 5.1 Top-Level Declarations / 顶层声明

**EN**
The following top-level declarations are expected in v0.x:
- `app(...) {}`
- `profile(...) {}`
- `capability(...) {}`
- `import(...)`
- optional `const(...)` / `var(...)`

**ZH**
v0.x 预计支持以下顶层声明：
- `app(...) {}`
- `profile(...) {}`
- `capability(...) {}`
- `import(...)`
- 可选的 `const(...)` / `var(...)`

## 5.2 Workload Declarations / 工作负载声明

**EN**
The language should expose workload kinds as first-class declarations:
- `service(...) {}`
- `worker(...) {}`
- `job(...) {}`

These are distinct at the language level even if they may later lower into a common internal workload model.

**ZH**
语言应将工作负载类型作为一等声明公开：
- `service(...) {}`
- `worker(...) {}`
- `job(...) {}`

即使它们在内部最终可能被 lowering 为统一的工作负载模型，在语言层也应保持区分。

---

# 6. Composition Model / 组合模型

## 6.1 Import / 导入

**EN**
`import(...)` is a module-system action, not a normal attribute. Imports participate in module graph construction and symbol resolution.

**ZH**
`import(...)` 是模块系统动作，而不是普通属性。导入行为将参与模块图构建与符号解析。

## 6.2 Profiles and Capabilities / Profile 与 Capability

**EN**
Profiles represent reusable deploy patterns. Capabilities represent composable feature bundles or attachable deploy behaviors.

**ZH**
Profile 表示可复用的部署模式。Capability 表示可组合的能力包或可附加的部署行为。

## 6.3 Use / 使用

**EN**
Composition is explicit via `use(...)`. Example:

```kotlin
use(profile("java-web")) {
  port = 8080
}

use(capability("redis-client"))
```

`use` should support parameter binding with typed validation.

**ZH**
组合通过 `use(...)` 显式完成。例如：

```kotlin
use(profile("java-web")) {
  port = 8080
}

use(capability("redis-client"))
```

`use` 应支持带类型校验的参数绑定。

## 6.4 Merge Rules / 合并规则

**EN**
The compiler must define deterministic merge rules:
- Scalar fields: later override earlier.
- Maps: key-wise merge, later values override conflicts.
- Structured blocks: recursive merge by schema.
- Illegal conflict cases must produce diagnostics.

**ZH**
编译器必须定义确定性的合并规则：
- 标量字段：后者覆盖前者。
- Map：按 key 合并，同 key 后者覆盖。
- 结构化 block：按 schema 递归合并。
- 非法冲突必须生成诊断信息。

---

# 7. Runtime and Run Model / 运行时与 Run 模型

## 7.1 Design Direction / 设计方向

**EN**
`run` should be modeled as a strongly typed child scope of a workload rather than as a stringly-typed statement parameter.

**ZH**
`run` 应被建模为工作负载下的**强类型子作用域**，而不是字符串参数驱动的语法动作。

## 7.2 Surface Syntax Direction / 表层语法方向

**EN**
Preferred direction:

```kotlin
service("api") {
  run {
    app {
      image = image("registry.local/api:1.0.0")
    }
  }
}
```

This keeps `run` as a typed field scope and allows future extension for named runs, sidecars, or init units.

**ZH**
推荐方向：

```kotlin
service("api") {
  run {
    app {
      image = image("registry.local/api:1.0.0")
    }
  }
}
```

这种方式将 `run` 保留为类型化字段作用域，并为未来扩展命名运行单元、sidecar、init unit 等能力预留空间。

## 7.3 Semantic Model / 语义模型

**EN**
A normalized semantic model may lower this into something conceptually equivalent to:
- `RunSpec::App(AppRunSpec)`
- `RunSpec::Process(ProcessRunSpec)`
- `RunSpec::Firecracker(FirecrackerRunSpec)`

Exact names are implementation details.

**ZH**
规范化后的语义模型可将其 lowering 为概念上等价于：
- `RunSpec::App(AppRunSpec)`
- `RunSpec::Process(ProcessRunSpec)`
- `RunSpec::Firecracker(FirecrackerRunSpec)`

具体命名属于实现细节。

---

# 8. Lifecycle and Controlled Flow / 生命周期与受控流程

## 8.1 Rationale / 设计动机

**EN**
Deploy workflows often require conditional initialization, migration, validation, and gating. The DSL must support lifecycle control flow, but only in a restricted, domain-specific manner.

**ZH**
部署流程经常需要条件初始化、迁移、校验和门禁控制。因此 DSL 必须支持生命周期控制流，但这种控制流必须是**受限的、面向领域的**。

## 8.2 Design Principle / 设计原则

**EN**
The language must not become a general-purpose scripting environment. Control flow is allowed only within lifecycle-oriented contexts and must compile into a static workflow IR.

**ZH**
语言不能演变为通用脚本环境。控制流只能出现在生命周期相关上下文中，并且必须可编译为静态 workflow IR。

## 8.3 Allowed Contexts / 允许的上下文

**EN**
Examples of allowed contexts:
- `init {}`
- `lifecycle {}`
- `hook {}`
- `step {}`

**ZH**
允许控制流出现的上下文示例：
- `init {}`
- `lifecycle {}`
- `hook {}`
- `step {}`

## 8.4 Condition Sources / 条件来源

**EN**
Conditions should be built from built-in check APIs and restricted boolean expressions.

Examples:
- `databaseSchema("main").isValid()`
- `service("postgres").healthy()`
- `httpProbe("http://x/health").status == 200`

**ZH**
条件应由内建 check API 与受限布尔表达式构成。

例如：
- `databaseSchema("main").isValid()`
- `service("postgres").healthy()`
- `httpProbe("http://x/health").status == 200`

## 8.5 Allowed Actions / 允许的动作

**EN**
Actions inside lifecycle control flow should be limited to domain actions such as:
- `skip()`
- `run("migrate")`
- `fail("...")`
- `waitUntil(...)`
- `retry(...)`

**ZH**
生命周期控制流中的动作应限制为领域动作，例如：
- `skip()`
- `run("migrate")`
- `fail("...")`
- `waitUntil(...)`
- `retry(...)`

## 8.6 Example / 示例

```kotlin
service("api") {
  init("schema") {
    if (databaseSchema("main").isValid()) {
      skip()
    } else {
      run("migrate")
    }
  }
}
```

**EN**  
This is acceptable only if `if` is restricted to lifecycle scopes and lowers into a static condition-action workflow node.

**ZH**  
该写法只有在 `if` 被限制在生命周期作用域中，且最终会 lowering 为静态条件-动作工作流节点时才是可接受的。

---

# 9. Check API / 检查 API

## 9.1 Purpose / 目的

**EN**
Check APIs provide read-only environmental or dependency checks used by lifecycle gates and conditional execution.

**ZH**
Check API 提供只读的环境或依赖检查能力，用于生命周期门禁和条件执行。

## 9.2 Requirements / 要求

**EN**
Check APIs must be:
- Read-only
- Side-effect free
- Timeout-aware
- Diagnosable
- Lowerable into workflow IR

**ZH**
Check API 必须满足：
- 只读
- 无副作用
- 可设置超时
- 可诊断
- 可 lowering 为 workflow IR

## 9.3 Initial Built-ins / 初始内建项

**EN**
Recommended initial built-ins:
- `databaseSchema(name)`
- `databaseConnection(name)`
- `service(name)` readiness/health checks
- `httpProbe(url)`
- `tcpProbe(host, port)`
- `fileExists(path)`
- `secretExists(name)`

**ZH**
建议的初始内建检查项：
- `databaseSchema(name)`
- `databaseConnection(name)`
- `service(name)` 就绪/健康检查
- `httpProbe(url)`
- `tcpProbe(host, port)`
- `fileExists(path)`
- `secretExists(name)`

---

# 10. Compiler Architecture / 编译器架构

## 10.1 Core Positioning / 核心定位

**EN**
The implementation must be a compile engine rather than a plain config loader.

**ZH**
实现层必须是一个 compile engine，而不是普通配置加载器。

## 10.2 Pipeline / 管线

**EN**
Recommended compilation pipeline:
1. Lexer
2. Parser
3. AST
4. Module Loader
5. Module Graph
6. Symbol Resolution
7. Semantic Analysis
8. Normalization / Expansion
9. Normalized Deploy Model
10. Plan IR
11. Backend Lowering
12. Backend IR
13. Apply / Executor

**ZH**
推荐的编译管线：
1. Lexer
2. Parser
3. AST
4. Module Loader
5. Module Graph
6. Symbol Resolution
7. Semantic Analysis
8. Normalization / Expansion
9. Normalized Deploy Model
10. Plan IR
11. Backend Lowering
12. Backend IR
13. Apply / Executor

## 10.3 Required IR Layers / 必需的 IR 分层

**EN**
At minimum the system should define:
- AST (syntax-oriented)
- Semantic Model (typed deploy meaning)
- Plan IR (deployment actions and graph)
- Backend IR (runtime/backend-specific lowering)

**ZH**
系统至少应定义：
- AST（面向语法）
- Semantic Model（面向强类型部署语义）
- Plan IR（面向部署动作与图）
- Backend IR（面向运行时/后端 lowering）

## 10.4 Explainability / 可解释性

**EN**
The compiler should maintain origin traces for effective values, including:
- imported source
- profile/capability application
- override source
- default source

**ZH**
编译器应维护生效值的来源追踪，包括：
- import 来源
- profile/capability 应用来源
- override 来源
- 默认值来源

---

# 11. Tooling Model / 工具体系

## 11.1 CLI Modes / CLI 模式

**EN**
Recommended commands:
- `parse`
- `check`
- `render`
- `plan`
- `apply`
- `explain`

**ZH**
推荐命令：
- `parse`
- `check`
- `render`
- `plan`
- `apply`
- `explain`

## 11.2 LSP / 语言服务器

**EN**
The LSP should reuse frontend and semantic layers, not runtime backend logic. Initial LSP capabilities should include:
- diagnostics
- completion
- hover
- go to definition
- document symbols

**ZH**
LSP 应复用 frontend 与 semantic 层，而不是 runtime backend 逻辑。初始 LSP 能力建议包括：
- diagnostics
- completion
- hover
- go to definition
- document symbols

---

# 12. Crate and Module Boundaries / Crate 与模块边界

## 12.1 Architectural Direction / 架构方向

**EN**
The language subsystem should be split into frontend, compiler core, and tooling-oriented crates rather than a single overloaded `warden-dsl` crate.

**ZH**
语言子系统应拆分为 frontend、compiler core、tooling 三类 crate，而不是继续维持一个过载的 `warden-dsl` 单 crate。

## 12.2 Recommended Split / 推荐拆分

**EN**
Suggested crate directions:
- `warden-dsl-frontend`
- `warden-dsl-compiler`
- `warden-plan`
- `orch-cli`
- `warden-lsp`

Possible finer-grained internal crates later:
- `warden-dsl-ast`
- `warden-dsl-parser`
- `warden-dsl-diagnostics`
- `warden-dsl-module`
- `warden-dsl-semantic`
- `warden-dsl-normalize`
- `warden-plan-types`

**ZH**
推荐的 crate 方向：
- `warden-dsl-frontend`
- `warden-dsl-compiler`
- `warden-plan`
- `orch-cli`
- `warden-lsp`

后续可进一步细分为：
- `warden-dsl-ast`
- `warden-dsl-parser`
- `warden-dsl-diagnostics`
- `warden-dsl-module`
- `warden-dsl-semantic`
- `warden-dsl-normalize`
- `warden-plan-types`

## 12.3 Dependency Direction / 依赖方向

**EN**
Dependency direction must remain one-way:
- frontend -> compiler -> plan -> backend/runtime
- frontend -> lsp
- cli may depend on all upper layers as an entrypoint

Language layers must not depend directly on runtime implementation details.

**ZH**
依赖方向必须保持单向：
- frontend -> compiler -> plan -> backend/runtime
- frontend -> lsp
- cli 作为入口可以依赖上层所有能力

语言层不得直接依赖具体 runtime 实现细节。

---

# 13. Versioning and Evolution / 版本与演进

## 13.1 v0.1 Scope / v0.1 范围

**EN**
Recommended v0.1 scope:
- app / service / worker / job
- import / use
- run
- resources
- expose
- health
- storage
- dependency
- lifecycle init with restricted control flow
- check/render/plan-ready compiler frontend

**ZH**
建议的 v0.1 范围：
- app / service / worker / job
- import / use
- run
- resources
- expose
- health
- storage
- dependency
- 带受限控制流的 lifecycle init
- 具备 check/render/plan 基础能力的 compiler frontend

## 13.2 Later Versions / 后续版本

**EN**
Potential later features:
- richer package system
- environment overlays
- named run units / sidecars / init containers
- autoscaling
- canary / blue-green rollout
- incremental compilation database
- formatter
- advanced LSP refactors

**ZH**
后续可扩展能力：
- 更完整的 package system
- 环境覆盖层
- 命名运行单元 / sidecar / init containers
- autoscaling
- canary / blue-green rollout
- 增量编译数据库
- formatter
- 高级 LSP 重构能力

---

# 14. Summary / 总结

**EN**  
Warden Deploy DSL is a strongly-typed, deploy-focused vertical DSL with Kotlin-DSL-like authoring experience, compile-time semantics, controlled lifecycle flow, and multi-runtime lowering. Its long-term value lies not in configuration syntax alone but in becoming the deploy language and compilation core for the Warden runtime platform.

**ZH**  
Warden Deploy DSL 是一门强类型、面向部署场景的垂直 DSL，具备 Kotlin DSL 风格的编写体验、编译期语义校验、受控生命周期流程能力，以及面向多运行时的 lowering 能力。它的长期价值不在于“配置格式”本身，而在于成为 Warden 平台的部署语言与编译核心。

