```text
app("mall") {
    let env = "prod"
    let version = "1.2.3"

    services {
        val redis = create("redis") {
            runtime(container)
            image("redis:7")
            expose("redis") {
                container(6379)
            }
        }

        val postgres = create("postgres") {
            runtime(container)
            image("postgres:16")
            expose("db") {
                container(5432)
            }
        }

        val gateway = create("gateway") {
            runtime(container)
            image("ghcr.io/acme/gateway:${version}")
            replicas(if env == "prod" then 3 else 1)

            dependsOn(redis, postgres)

            env {
                set("REDIS_ADDR", redis.endpoint("redis"))
                set("DB_ADDR", postgres.endpoint("db"))
            }

            resources {
                cpu(500.milli)
                memory(512.Mi)
            }

            healthcheck {
                http {
                    path("/health")
                    port(http)
                }
            }
        }
    }

    ingress("gateway-public") {
        host("mall.example.com")
        route("/") {
            backend(services.gateway)
            port("http")
        }
    }
}
```

1. 设计目标

这门 DSL 的目标不是“像配置文件”，而是：

像 Gradle Kotlin DSL 一样，通过受控的 block / invocation / scoped accessor 来构建一个强类型的编排对象图。

核心追求：

语法上接近 Gradle/Kotlin DSL

语义上强约束、强类型、低字符串依赖

适合 plan / validate / diff / apply

适合后续接 script engine

适合做 IDE/LSP 补全

适合长期演进，而不是只做配置解析

2. 总体设计原则
   2.1 主体是“对象构建 DSL”

不是 key-value 配置文件。

优先：

service("gateway") {
runtime(container)
replicas(3)
}

不优先：

service "gateway" {
runtime = "container"
replicas = 3
}
2.2 引用尽量强类型化

优先：

dependsOn(services.redis, services.postgres)
backend(services.gateway)
use(secrets.dbPassword)

不优先：

dependsOn("redis", "postgres")
backend("gateway")
use("dbPassword")
2.3 作用域就是语义边界

进入某个 block 后，只暴露该对象能配置的成员。

例如：

service {} 内只能写 service 相关配置

resources {} 内只能写资源相关配置

healthcheck {} 内只能写健康检查相关配置

2.4 动态能力受控

表达式可以有，但必须受约束。
脚本引擎可以接，但不是主语法层。

3. 语言风格
   3.1 顶层风格

采用 invocation-style DSL：

app("mall") {
...
}
3.2 配置项风格

采用函数调用式：

runtime(container)
image("ghcr.io/acme/gateway:1.2.3")
replicas(3)
3.3 嵌套对象风格

采用 block：

resources {
cpu(500.milli)
memory(512.Mi)
}
3.4 命名实体风格

命名对象统一：

service("gateway") { ... }
port("http") { ... }
config("app") { ... }
4. 顶层对象模型

v0.1 建议支持这些顶层 block：

app(...)

let ...

services { ... }

configs { ... }

secrets { ... }

volumes { ... }

networks { ... }

ingress(...) { ... }

task(...) { ... }

policy(...) { ... }

hook(...) { ... }

其中最核心的是：

app

services

configs

secrets

ingress

task

5. 变量与常量
   5.1 变量声明

使用 let：

let env = "prod"
let version = "1.2.3"

含义：

绑定一个不可变局部符号

可在当前作用域及子作用域中引用

由 binder 做作用域解析

6. 强类型 accessors 设计

这是整个 DSL 最重要的一部分。

6.1 命名空间

系统内置以下 accessor namespace：

services

configs

secrets

volumes

networks

tasks

policies

例如：

services.gateway
services.redis
secrets.dbPassword
configs.app
6.2 accessor 的语义

它们不是动态 map 访问，而是：

语义层解析的命名对象引用

binder 负责把它们绑定为 typed symbol ref

例如：

services.gateway -> ServiceRef("gateway")

secrets.dbPassword -> SecretRef("dbPassword")

6.3 accessor 的类型约束

不同 API 只接受对应类型：

dependsOn(...) 只接受 ServiceRef

backend(...) 只接受 ServiceRef

use(...) 只接受 SecretRef | ConfigRef

mount(...) 只接受 VolumeRef

7. 类型化字面量

v0.1 建议支持三类。

7.1 duration
10.seconds
3.seconds
5.minutes
7.2 cpu
500.milli
1.cpu
2.cpu
7.3 memory
512.Mi
1.Gi
4.Gi

这类字面量在 parser 层可先解析成：

IntLiteral(500) + member accessor milli

再在 binder/type checker 层转换为特定 typed literal

8. 枚举 symbol

为了减少字符串，建议一些枚举值直接作为 symbol：

container

process

firecracker

always

onFailure

never

tcp

udp

http

例如：

runtime(container)
restartPolicy(always)
protocol(tcp)

而不是：

runtime("container")
restartPolicy("always")
protocol("tcp")
9. 语法草案

下面是一版简化 EBNF 风格描述。

9.1 文件
File            := AppDecl EOF ;
9.2 app
AppDecl          := "app" "(" StringLiteral ")" Block ;
9.3 block
Block            := "{" Statement* "}" ;
9.4 statement
Statement        := LetDecl
| ServicesBlock
| ConfigsBlock
| SecretsBlock
| VolumesBlock
| NetworksBlock
| IngressDecl
| TaskDecl
| PolicyDecl
| HookDecl
| Invocation
;
9.5 let
LetDecl          := "let" Identifier "=" Expression ;
9.6 services
ServicesBlock    := "services" Block ;
ServiceDecl      := "service" "(" StringLiteral ")" Block ;
9.7 invocation
Invocation       := Identifier "(" ArgList? ")" ;
ArgList          := Expression ("," Expression)* ;
9.8 accessor
AccessorExpr     := Identifier ("." Identifier)+ ;
9.9 expression
Expression       := Literal
| Identifier
| AccessorExpr
| IfExpr
| Invocation
| InterpolatedString
| BinaryExpr
;
9.10 if expression
IfExpr           := "if" Expression "then" Expression "else" Expression ;

v0.1 先用单行表达式式的 if ... then ... else ...，比块状 if 更好解析，也更适合作为表达式。

10. 核心对象 DSL 设计
    10.1 app
    app("mall") {
    let env = "prod"
    let version = "1.2.3"
    }

含义：

声明一个应用

顶层作用域

后续所有命名对象都注册到 app symbol table

10.2 services
services {
service("gateway") {
runtime(container)
image("ghcr.io/acme/gateway:${version}")
replicas(if env == "prod" then 3 else 1)
}
}
10.3 service

推荐字段：

runtime(...)

image(...)

program(...)

args(...)

workingDir(...)

user(...)

replicas(...)

dependsOn(...)

restartPolicy(...)

env { ... }

port(...) { ... }

resources { ... }

healthcheck { ... }

mount(...)

use(...)

labels { ... }

示例
service("gateway") {
runtime(container)
image("ghcr.io/acme/gateway:${version}")
replicas(3)
dependsOn(services.redis, services.postgres)
restartPolicy(always)
}
10.4 env block
env {
set("APP_ENV", env)
set("LOG_LEVEL", "info")
}

这里不使用裸键值对，保持 DSL API 感。

10.5 port
port("http") {
container(8080)
expose(80)
protocol(tcp)
}

port("name") 返回 PortRef，但该 ref 默认作用域只在当前 service 内可见。
如果要跨对象引用，建议通过 services.gateway.port("http") 或 services.gateway.endpoint("http") 这种 accessor API。

10.6 resources
resources {
cpu(500.milli)
memory(512.Mi)
}
10.7 healthcheck
healthcheck {
http {
path("/health")
port("http")
}
interval(10.seconds)
timeout(3.seconds)
}

v0.1 里 port("http") 可以先保留字符串，因为它是 service 内部局部引用。
v0.2 可进一步升级为 typed local ref。

10.8 configs
configs {
config("app") {
data {
set("application.yaml", file("./config/app.yaml"))
}
}
}

configs.app 会绑定为 ConfigRef("app")。

service 中可引用：

use(configs.app)
10.9 secrets
secrets {
secret("dbPassword") {
fromEnv("DB_PASSWORD")
}
}

或：

secrets {
secret("dbPassword") {
value(env("DB_PASSWORD"))
}
}

service 中：

use(secrets.dbPassword)
10.10 volumes
volumes {
volume("data") {
driver(local)
}
}

service 中：

mount(volumes.data) {
at("/data")
}
10.11 ingress
ingress("public") {
host("mall.example.com")

    route("/") {
        backend(services.gateway)
        port("http")
    }
}

这里 backend(...) 必须是 ServiceRef。

10.12 task
task("migrate-db") {
runtime(container)
image("ghcr.io/acme/migrator:${version}")
runBefore(services.gateway)
}

或者：

task("migrate-db") {
runtime(container)
image("ghcr.io/acme/migrator:${version}")
phase(preApply)
}
10.13 policy / hook

先保留入口，不做过重设计。

policy("placement") {
engine(rune)
script(file("./policies/placement.rn"))
}
hook("preApplyCheck") {
on(preApply)
engine(rune)
script(file("./hooks/check_cluster.rn"))
}

这里是未来 script engine 接入点。

11. 表达式系统

v0.1 建议非常克制。

11.1 支持

string / int / bool literal

identifier

accessor

interpolated string

simple invocation

if cond then a else b

== != && ||

基础比较运算

11.2 不支持

loop

lambda

user-defined function

mutation

statement block expression

任意脚本

12. 完整示例

下面是一版更像你目标形态的完整草案。

app("mall") {
let env = "prod"
let version = "1.2.3"

    configs {
        config("gatewayConfig") {
            data {
                set("application.yaml", file("./configs/gateway.yaml"))
            }
        }
    }

    secrets {
        secret("dbPassword") {
            fromEnv("DB_PASSWORD")
        }
    }

    volumes {
        volume("redisData") {
            driver(local)
        }

        volume("postgresData") {
            driver(local)
        }
    }

    services {
        service("redis") {
            runtime(container)
            image("redis:7")

            port("redis") {
                container(6379)
            }

            mount(volumes.redisData) {
                at("/data")
            }

            resources {
                cpu(500.milli)
                memory(256.Mi)
            }
        }

        service("postgres") {
            runtime(container)
            image("postgres:16")

            env {
                set("POSTGRES_PASSWORD", secret(secrets.dbPassword))
            }

            port("db") {
                container(5432)
            }

            mount(volumes.postgresData) {
                at("/var/lib/postgresql/data")
            }

            resources {
                cpu(1.cpu)
                memory(1.Gi)
            }
        }

        service("gateway") {
            runtime(container)
            image("ghcr.io/acme/gateway:${version}")

            replicas(if env == "prod" then 3 else 1)
            dependsOn(services.redis, services.postgres)
            use(configs.gatewayConfig)

            env {
                set("APP_ENV", env)
                set("REDIS_ADDR", services.redis.endpoint("redis"))
                set("DB_ADDR", services.postgres.endpoint("db"))
                set("DB_PASSWORD", secret(secrets.dbPassword))
            }

            port("http") {
                container(8080)
                expose(80)
                protocol(tcp)
            }

            resources {
                cpu(1.cpu)
                memory(512.Mi)
            }

            healthcheck {
                http {
                    path("/health")
                    port("http")
                }
                interval(10.seconds)
                timeout(3.seconds)
            }

            restartPolicy(always)
        }
    }

    ingress("public") {
        host("mall.example.com")

        route("/") {
            backend(services.gateway)
            port("http")
        }
    }

    task("migrate-db") {
        runtime(container)
        image("ghcr.io/acme/migrator:${version}")
        runBefore(services.gateway)

        env {
            set("DB_ADDR", services.postgres.endpoint("db"))
            set("DB_PASSWORD", secret(secrets.dbPassword))
        }
    }

    hook("preApplyCheck") {
        on(preApply)
        engine(rune)
        script(file("./hooks/check_cluster.rn"))
    }
}
13. AST 草案

下面给你一版简化 AST。

pub struct FileAst {
pub app: AppAst,
}

pub struct AppAst {
pub name: String,
pub body: Vec<StmtAst>,
}

pub enum StmtAst {
Let(LetAst),
ServicesBlock(ServicesBlockAst),
ConfigsBlock(ConfigsBlockAst),
SecretsBlock(SecretsBlockAst),
VolumesBlock(VolumesBlockAst),
Ingress(IngressAst),
Task(TaskAst),
Policy(PolicyAst),
Hook(HookAst),
Invocation(InvocationAst),
}

表达式：

pub enum ExprAst {
String(String),
Int(i64),
Bool(bool),
Identifier(String),
Accessor(Vec<String>),      // services.gateway
InterpolatedString(Vec<InterpolatedPartAst>),
IfExpr {
cond: Box<ExprAst>,
then_expr: Box<ExprAst>,
else_expr: Box<ExprAst>,
},
Binary {
op: BinaryOp,
left: Box<ExprAst>,
right: Box<ExprAst>,
},
Invocation(InvocationAst),
}

invocation：

pub struct InvocationAst {
pub callee: String,
pub args: Vec<ExprAst>,
}

block object：

pub struct ServiceAst {
pub name: String,
pub body: Vec<ServiceStmtAst>,
}
14. Binder 设计草案

Binder 是这个 DSL 的核心之一。

14.1 它负责：

作用域解析

let 变量绑定

accessor 绑定

命名对象注册

引用类型检查前置解析

14.2 例子
dependsOn(services.redis, services.postgres)

binder 会把：

services.redis -> ResolvedRef::Service("redis")

services.postgres -> ResolvedRef::Service("postgres")

然后 dependsOn 的签名检查参数类型必须为 ServiceRef...

15. 类型系统草案

v0.1 不需要很重，但需要有。

建议内部有这些类型：

enum Type {
String,
Int,
Bool,
Duration,
Cpu,
Memory,
ServiceRef,
ConfigRef,
SecretRef,
VolumeRef,
PortRef,
RuntimeKind,
RestartPolicy,
Protocol,
Unit,
}

并且给 invocation 定义签名，例如：

runtime(RuntimeKind) -> Unit
image(String) -> Unit
replicas(Int) -> Unit
dependsOn(ServiceRef...) -> Unit
backend(ServiceRef) -> Unit
cpu(Cpu) -> Unit
memory(Memory) -> Unit
interval(Duration) -> Unit
timeout(Duration) -> Unit
16. IR 草案

IR 要脱离源语法。

pub struct AppIr {
pub name: String,
pub services: Vec<ServiceIr>,
pub configs: Vec<ConfigIr>,
pub secrets: Vec<SecretIr>,
pub volumes: Vec<VolumeIr>,
pub ingresses: Vec<IngressIr>,
pub tasks: Vec<TaskIr>,
pub hooks: Vec<HookIr>,
}

service IR：

pub struct ServiceIr {
pub name: String,
pub runtime: RuntimeKind,
pub image: Option<String>,
pub program: Option<String>,
pub replicas: u32,
pub depends_on: Vec<String>,
pub env: Vec<(String, EnvValueIr)>,
pub ports: Vec<PortIr>,
pub resources: Option<ResourceIr>,
pub mounts: Vec<MountIr>,
pub healthcheck: Option<HealthcheckIr>,
pub restart_policy: Option<RestartPolicy>,
}

注意：

到 IR 层时，services.redis 这类 accessor 不应再存在

应该已经 resolve 成稳定 ref/id

17. 设计边界
    17.1 v0.1 不做

泛型

用户自定义函数

复杂宏

循环

任意宿主调用

plugin system

跨文件 module system 的复杂设计

17.2 v0.1 要保证

语法风格明确

作用域清晰

引用强类型化

能顺利 lower 到 IR

能为 v0.2 接 script engine 留位置

18. 实现优先级

我建议按下面顺序落地。

Phase 1

先做这些：

app

let

services

service

runtime/image/replicas

dependsOn

env

port

resources

healthcheck

ingress

configs/secrets/volumes 的最小引用能力

binder + type checker + IR lowering

Phase 2

再做：

task

policy

hook

typed literal

endpoint(...)

mount/use/secret(...) 的细化

多文件 import

Phase 3

再接：

script engine

generator

policy execution

richer local refs

LSP

19. 这版草案的定位

这版 DSL 草案的核心风格可以概括为：

Gradle/Kotlin DSL 风格的 invocation-style 外部 DSL，带有 typesafe accessors、typed refs 和受控表达式系统。

它和 HCL 的差异在于：

更像对象建模

更少 key-value 表格感

更强调 scope / accessor / typed ref

更适合你后面做强类型编排器

20. 我对这版的建议

如果你认可这个方向，下一步最该做的不是继续加 feature，而是先把这三件事钉死：

1. invocation 列表

到底哪些函数是语言内建的

2. accessor namespace

services.xxx / secrets.xxx / configs.xxx 的最终规则

3. 表达式边界

到底只做有限表达式，还是要提前为 script expression 留语法接口