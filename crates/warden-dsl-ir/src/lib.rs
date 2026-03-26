//! Intermediate representation: workloads and ingress extracted from [`warden_dsl_hir::HirDocument`].
//!
//! This layer is intentionally small and JSON-friendly; it can be mapped to
//! `warden-dsl` manifests or API types in a later step.

use serde::{Deserialize, Serialize};
use warden_dsl_ast::{ExprAst, PathAst, StmtAst};
use warden_dsl_hir::HirDocument;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrApplication {
  pub app_name: String,
  pub volumes: Vec<IrVolume>,
  pub configs: Vec<IrConfig>,
  pub secrets: Vec<IrSecret>,
  pub workloads: Vec<IrWorkload>,
  pub ingress: Vec<IrIngress>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrVolume {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrConfig {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrSecret {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrWorkload {
  /// `val <binding> = create(...)`
  pub binding: String,
  pub name: String,
  #[serde(default)]
  pub depends_on: Vec<String>,
  pub kind: Option<String>,
  pub runtime: Option<String>,
  pub image: Option<String>,
  pub endpoint_name: Option<String>,
  pub endpoint_protocol: Option<String>,
  /// `endpoint(...) { port(...) }` or legacy `expose(...) { container(port) }`
  /// when present.
  #[serde(default)]
  pub service_port: Option<u16>,
  /// Unresolved replica expression as text (may contain `if` / interpolations).
  pub replicas: Option<String>,
  #[serde(default)]
  pub mounts: Vec<IrMount>,
  #[serde(default)]
  pub env: Vec<IrEnvVar>,
  pub resources: Option<IrResources>,
  pub health: Option<IrHealth>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IrMount {
  pub volume: String,
  pub target: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrEnvVar {
  pub name: String,
  pub value: ExprAst,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IrResources {
  pub cpu: Option<String>,
  pub memory: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IrHttpProbe {
  pub path: String,
  pub endpoint: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IrEndpointRef {
  pub workload: String,
  pub endpoint: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IrHealth {
  pub readiness: Option<IrHttpProbe>,
  pub liveness: Option<IrHttpProbe>,
  pub startup: Option<IrHttpProbe>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrIngress {
  pub name: String,
  pub host: Option<String>,
  #[serde(default)]
  pub routes: Vec<IrRoute>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrRoute {
  pub path: Option<String>,
  pub backend: Option<IrEndpointRef>,
}

/// Lower HIR into a compact IR snapshot.
pub fn lower(hir: &HirDocument) -> IrApplication {
  let volumes = hir
    .volumes
    .iter()
    .map(|v| IrVolume {
      name: v.name.clone(),
    })
    .collect();
  let configs = hir
    .configs
    .iter()
    .map(|c| IrConfig {
      name: c.name.clone(),
    })
    .collect();
  let secrets = hir
    .secrets
    .iter()
    .map(|s| IrSecret {
      name: s.name.clone(),
    })
    .collect();
  let workloads = hir
    .services
    .iter()
    .map(|s| IrWorkload {
      binding: s.binding.to_string(),
      name: s.name.clone(),
      depends_on: extract_depends_on(&s.body),
      kind: find_unary_invoke_text(&s.body, "kind"),
      runtime: find_unary_invoke_text(&s.body, "runtime"),
      image: find_string_first_arg(&s.body, "image"),
      endpoint_name: endpoint_name(&s.body),
      endpoint_protocol: endpoint_protocol(&s.body),
      service_port: endpoint_port(&s.body),
      replicas: find_replicas(&s.body),
      mounts: extract_mounts(&s.body),
      env: extract_env(&s.body),
      resources: extract_resources(&s.body),
      health: extract_health(&s.body),
    })
    .collect();
  let ingress = hir
    .ingress_blocks
    .iter()
    .map(|b| IrIngress {
      name: b.name.clone(),
      host: find_string_first_arg(&b.body, "host"),
      routes: extract_routes(&b.body),
    })
    .collect();
  IrApplication {
    app_name: hir.app_name.clone(),
    volumes,
    configs,
    secrets,
    workloads,
    ingress,
  }
}

fn single_segment(path: &PathAst, want: &str) -> bool {
  path.segments.len() == 1 && path.segments[0].as_str() == want
}

fn extract_depends_on(body: &[StmtAst]) -> Vec<String> {
  let mut out = Vec::new();
  for stmt in body {
    let StmtAst::Invoke(invoke) = stmt else {
      continue;
    };
    if !single_segment(&invoke.callee, "dependsOn") {
      continue;
    }
    for arg in &invoke.args {
      match arg {
        ExprAst::Identifier(v) => out.push(v.to_string()),
        ExprAst::Path(path) => {
          if let Some(last) = path.segments.last() {
            out.push(last.to_string());
          }
        }
        _ => {}
      }
    }
  }
  out
}

fn find_string_first_arg(body: &[StmtAst], name: &str) -> Option<String> {
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if !single_segment(&inv.callee, name) {
      continue;
    }
    if let Some(ExprAst::String(s)) = inv.args.first() {
      return Some(s.clone());
    }
  }
  None
}

fn find_unary_invoke_text(body: &[StmtAst], name: &str) -> Option<String> {
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if !single_segment(&inv.callee, name) {
      continue;
    }
    return inv.args.first().map(expr_to_text);
  }
  None
}

fn endpoint_port(body: &[StmtAst]) -> Option<u16> {
  let mut saw_endpoint = false;
  for stmt in body {
    let StmtAst::Block(b) = stmt else {
      continue;
    };
    if !single_segment(&b.callee, "endpoint") {
      continue;
    }
    saw_endpoint = true;
    for inner in &b.body {
      let StmtAst::Invoke(inv) = inner else {
        continue;
      };
      if !single_segment(&inv.callee, "port") {
        continue;
      }
      if let Some(ExprAst::Integer(p)) = inv.args.first() {
        return u16::try_from(*p).ok();
      }
    }
  }
  if saw_endpoint {
    return None;
  }
  legacy_expose_container_port(body)
}

fn legacy_expose_container_port(body: &[StmtAst]) -> Option<u16> {
  for stmt in body {
    let StmtAst::Block(b) = stmt else {
      continue;
    };
    if !single_segment(&b.callee, "expose") {
      continue;
    }
    for inner in &b.body {
      let StmtAst::Invoke(inv) = inner else {
        continue;
      };
      if !single_segment(&inv.callee, "container") {
        continue;
      }
      if let Some(ExprAst::Integer(p)) = inv.args.first() {
        return u16::try_from(*p).ok();
      }
    }
  }
  None
}

fn endpoint_name(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "endpoint") {
      continue;
    }
    if let Some(ExprAst::String(name)) = block.args.first() {
      return Some(name.clone());
    }
  }
  legacy_expose_name(body)
}

fn endpoint_protocol(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "endpoint") {
      continue;
    }
    for inner in &block.body {
      let StmtAst::Invoke(inv) = inner else {
        continue;
      };
      if !single_segment(&inv.callee, "protocol") {
        continue;
      }
      return inv.args.first().map(expr_to_text);
    }
  }
  None
}

fn legacy_expose_name(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "expose") {
      continue;
    }
    if let Some(ExprAst::String(name)) = block.args.first() {
      return Some(name.clone());
    }
  }
  None
}

fn find_replicas(body: &[StmtAst]) -> Option<String> {
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if !single_segment(&inv.callee, "replicas") {
      continue;
    }
    return inv.args.first().map(expr_to_text);
  }
  None
}

fn find_stringy_invoke_text(body: &[StmtAst], name: &str) -> Option<String> {
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if !single_segment(&inv.callee, name) {
      continue;
    }
    return inv.args.first().map(expr_to_text);
  }
  None
}

fn extract_mounts(body: &[StmtAst]) -> Vec<IrMount> {
  let mut mounts = Vec::new();
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if !single_segment(&inv.callee, "mount") {
      continue;
    }
    let Some(volume) = inv.args.first().map(expr_to_text) else {
      continue;
    };
    let Some(ExprAst::String(target)) = inv.args.get(1) else {
      continue;
    };
    mounts.push(IrMount {
      volume,
      target: target.clone(),
    });
  }
  mounts
}

fn extract_env(body: &[StmtAst]) -> Vec<IrEnvVar> {
  let mut vars = Vec::new();
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "env") {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(inv) = nested else {
        continue;
      };
      if !single_segment(&inv.callee, "set") {
        continue;
      }
      let Some(ExprAst::String(name)) = inv.args.first() else {
        continue;
      };
      let Some(value) = inv.args.get(1) else {
        continue;
      };
      vars.push(IrEnvVar {
        name: name.clone(),
        value: value.clone(),
      });
    }
  }
  vars
}

fn extract_resources(body: &[StmtAst]) -> Option<IrResources> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "resources") {
      continue;
    }
    return Some(IrResources {
      cpu: find_stringy_invoke_text(&block.body, "cpu"),
      memory: find_stringy_invoke_text(&block.body, "memory"),
    });
  }
  None
}

fn extract_health(body: &[StmtAst]) -> Option<IrHealth> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, "health") {
      continue;
    }
    return Some(IrHealth {
      readiness: extract_probe(&block.body, "readiness"),
      liveness: extract_probe(&block.body, "liveness"),
      startup: extract_probe(&block.body, "startup"),
    });
  }
  None
}

fn extract_probe(body: &[StmtAst], stage: &str) -> Option<IrHttpProbe> {
  for stmt in body {
    let StmtAst::Block(block) = stmt else {
      continue;
    };
    if !single_segment(&block.callee, stage) {
      continue;
    }
    for nested in &block.body {
      let StmtAst::Invoke(inv) = nested else {
        continue;
      };
      if !single_segment(&inv.callee, "http") {
        continue;
      }
      let Some(ExprAst::String(path)) = inv.args.first() else {
        continue;
      };
      let Some(endpoint) = inv.args.get(1).map(expr_to_text) else {
        continue;
      };
      return Some(IrHttpProbe {
        path: path.clone(),
        endpoint,
      });
    }
  }
  None
}

fn extract_routes(body: &[StmtAst]) -> Vec<IrRoute> {
  let mut routes = Vec::new();
  for stmt in body {
    let StmtAst::Block(b) = stmt else {
      continue;
    };
    if !single_segment(&b.callee, "route") {
      continue;
    }
    let path = match b.args.first() {
      Some(ExprAst::String(s)) => Some(s.clone()),
      _ => None,
    };
    let backend = route_backend_ref(&b.body);
    routes.push(IrRoute { path, backend });
  }
  routes
}

fn route_backend_ref(body: &[StmtAst]) -> Option<IrEndpointRef> {
  let mut backend = None;
  let mut port = None;
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if single_segment(&inv.callee, "backend") {
      if let Some(expr) = inv.args.first() {
        match expr {
          ExprAst::Invocation { callee, args }
            if callee
              .segments
              .last()
              .is_some_and(|segment| segment == "endpoint") =>
          {
            let workload = extract_endpoint_workload(callee)?;
            let endpoint = match args.first() {
              Some(ExprAst::String(value)) => Some(value.clone()),
              _ => None,
            };
            backend = Some(IrEndpointRef { workload, endpoint });
          }
          _ => {
            let workload = extract_workload_ref(expr)?;
            backend = Some(IrEndpointRef {
              workload,
              endpoint: None,
            });
          }
        }
      }
    } else if single_segment(&inv.callee, "port") {
      port = inv.args.first().map(expr_to_text);
    }
  }
  backend.map(|mut value| {
    if value.endpoint.is_none() {
      value.endpoint = port.filter(|entry| !entry.trim().is_empty());
    }
    value
  })
}

fn extract_endpoint_workload(callee: &PathAst) -> Option<String> {
  if callee.segments.len() < 2 {
    return None;
  }
  let workload_segments = &callee.segments[..(callee.segments.len() - 1)];
  Some(
    workload_segments
      .iter()
      .map(|segment| segment.as_str())
      .collect::<Vec<_>>()
      .join("."),
  )
}

fn extract_workload_ref(expr: &ExprAst) -> Option<String> {
  match expr {
    ExprAst::Identifier(value) => Some(value.to_string()),
    ExprAst::Path(path) => Some(
      path
        .segments
        .iter()
        .map(|segment| segment.as_str())
        .collect::<Vec<_>>()
        .join("."),
    ),
    _ => None,
  }
}

fn expr_to_text(expr: &ExprAst) -> String {
  match expr {
    ExprAst::String(s) => s.clone(),
    ExprAst::Integer(i) => i.to_string(),
    ExprAst::Identifier(i) => i.to_string(),
    ExprAst::Path(p) => p
      .segments
      .iter()
      .map(|s| s.as_str())
      .collect::<Vec<_>>()
      .join("."),
    ExprAst::MemberNumber { value, unit } => format!("{}.{}", value, unit),
    ExprAst::IfEq {
      left,
      right,
      then_expr,
      else_expr,
    } => format!(
      "if {} == {:?} then {} else {}",
      left,
      right,
      expr_to_text(then_expr),
      expr_to_text(else_expr)
    ),
    ExprAst::Invocation { callee, args } => {
      let mut out = String::new();
      out.push_str(&path_text(callee));
      out.push('(');
      for (idx, a) in args.iter().enumerate() {
        if idx > 0 {
          out.push_str(", ");
        }
        out.push_str(&expr_to_text(a));
      }
      out.push(')');
      out
    }
  }
}

fn path_text(path: &PathAst) -> String {
  path
    .segments
    .iter()
    .map(|s| s.as_str())
    .collect::<Vec<_>>()
    .join(".")
}

#[cfg(test)]
mod tests {
  use super::*;
  use warden_dsl_hir::lower as lower_hir;
  use warden_dsl_parser::parse;

  #[test]
  fn lowers_gateway_and_ingress() {
    let raw = r#"
app("mall") {
  let env = "prod"
  volume("redisData") {}
  config("appConfig") {}
  secret("dbPassword") {}
  workload("gateway") {
    kind(worker)
    runtime(container)
    image("ghcr.io/x:${version}")
    replicas(if env == "prod" then 3 else 1)
    dependsOn(workloads.redis)
    mount(volumes.redisData, "/data")
    env {
      set("APP_CONFIG", configs.appConfig)
      set("DB_PASSWORD", secrets.dbPassword)
      set("REDIS_ADDR", workloads.redis.endpoint("redis"))
      set("MODE", "prod")
    }
    endpoint("http") { port(8080) protocol(http) }
    resources {
      cpu(500.milliCpu)
      memory(512.mebi)
    }
    health {
      readiness { http("/ready", endpoint("http")) }
      liveness { http("/live", workloads.gateway.endpoint("http")) }
    }
  }
  ingress("gw") {
    host("mall.example.com")
    route("/") { backend(workloads.gateway.endpoint("http")) }
  }
  workload("redis") { runtime(containerd) }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let ir = lower(&hir);
    assert_eq!(ir.app_name, "mall");
    assert_eq!(ir.volumes[0].name, "redisData");
    assert_eq!(ir.configs[0].name, "appConfig");
    assert_eq!(ir.secrets[0].name, "dbPassword");
    assert_eq!(ir.workloads.len(), 2);
    let w = &ir.workloads[0];
    assert_eq!(w.binding, "gateway");
    assert_eq!(w.name, "gateway");
    assert_eq!(w.kind.as_deref(), Some("worker"));
    assert_eq!(w.runtime.as_deref(), Some("container"));
    assert_eq!(w.image.as_deref(), Some("ghcr.io/x:${version}"));
    assert_eq!(w.endpoint_name.as_deref(), Some("http"));
    assert_eq!(w.endpoint_protocol.as_deref(), Some("http"));
    assert_eq!(w.service_port, Some(8080));
    assert!(w.replicas.as_ref().is_some_and(|r| r.contains("if env")));
    assert_eq!(w.depends_on, vec!["redis".to_string()]);
    assert_eq!(w.mounts[0].volume, "volumes.redisData");
    assert_eq!(w.mounts[0].target, "/data");
    assert_eq!(w.env.len(), 4);
    assert_eq!(w.env[0].name, "APP_CONFIG");
    assert_eq!(w.env[1].name, "DB_PASSWORD");
    assert_eq!(w.env[2].name, "REDIS_ADDR");
    assert_eq!(w.env[3].name, "MODE");
    assert_eq!(
      w.resources.as_ref().and_then(|value| value.cpu.as_deref()),
      Some("500.milliCpu")
    );
    assert_eq!(
      w.resources
        .as_ref()
        .and_then(|value| value.memory.as_deref()),
      Some("512.mebi")
    );
    assert_eq!(
      w.health
        .as_ref()
        .and_then(|value| value.readiness.as_ref())
        .map(|value| value.path.as_str()),
      Some("/ready")
    );
    assert_eq!(
      w.health
        .as_ref()
        .and_then(|value| value.liveness.as_ref())
        .map(|value| value.endpoint.as_str()),
      Some("workloads.gateway.endpoint(http)")
    );
    assert_eq!(ir.ingress.len(), 1);
    let ing = &ir.ingress[0];
    assert_eq!(ing.name, "gw");
    assert_eq!(ing.host.as_deref(), Some("mall.example.com"));
    assert_eq!(ing.routes.len(), 1);
    assert_eq!(ing.routes[0].path.as_deref(), Some("/"));
    assert_eq!(
      ing.routes[0]
        .backend
        .as_ref()
        .map(|value| value.workload.as_str()),
      Some("workloads.gateway")
    );
    assert_eq!(
      ing.routes[0]
        .backend
        .as_ref()
        .and_then(|value| value.endpoint.as_deref()),
      Some("http")
    );
  }
}
