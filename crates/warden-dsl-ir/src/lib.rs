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
  pub workloads: Vec<IrWorkload>,
  pub ingress: Vec<IrIngress>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IrWorkload {
  /// `val <binding> = create(...)`
  pub binding: String,
  pub name: String,
  #[serde(default)]
  pub depends_on: Vec<String>,
  pub runtime: Option<String>,
  pub image: Option<String>,
  /// `expose(...) { container(port) }` when present.
  #[serde(default)]
  pub service_port: Option<u16>,
  /// Unresolved replica expression as text (may contain `if` / interpolations).
  pub replicas: Option<String>,
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
  pub backend: Option<String>,
  pub port: Option<String>,
}

/// Lower HIR into a compact IR snapshot.
pub fn lower(hir: &HirDocument) -> IrApplication {
  let workloads = hir
    .services
    .iter()
    .map(|s| IrWorkload {
      binding: s.binding.to_string(),
      name: s.name.clone(),
      depends_on: extract_depends_on(&s.body),
      runtime: find_unary_invoke_text(&s.body, "runtime"),
      image: find_string_first_arg(&s.body, "image"),
      service_port: expose_container_port(&s.body),
      replicas: find_replicas(&s.body),
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

fn expose_container_port(body: &[StmtAst]) -> Option<u16> {
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
    let (backend, port) = route_backend_port(&b.body);
    routes.push(IrRoute {
      path,
      backend,
      port,
    });
  }
  routes
}

fn route_backend_port(body: &[StmtAst]) -> (Option<String>, Option<String>) {
  let mut backend = None;
  let mut port = None;
  for stmt in body {
    let StmtAst::Invoke(inv) = stmt else {
      continue;
    };
    if single_segment(&inv.callee, "backend") {
      backend = inv.args.first().map(expr_to_text);
    } else if single_segment(&inv.callee, "port") {
      port = inv.args.first().map(expr_to_text);
    }
  }
  (backend, port)
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
  services {
    val gateway = create("gateway") {
      runtime(container)
      image("ghcr.io/x:${version}")
      replicas(if env == "prod" then 3 else 1)
      dependsOn(redis)
    }
  }
  ingress("gw") {
    host("mall.example.com")
    route("/") { backend(services.gateway) port("http") }
  }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower_hir(&ast).unwrap();
    let ir = lower(&hir);
    assert_eq!(ir.app_name, "mall");
    assert_eq!(ir.workloads.len(), 1);
    let w = &ir.workloads[0];
    assert_eq!(w.binding, "gateway");
    assert_eq!(w.name, "gateway");
    assert_eq!(w.runtime.as_deref(), Some("container"));
    assert_eq!(w.image.as_deref(), Some("ghcr.io/x:${version}"));
    assert!(w.replicas.as_ref().is_some_and(|r| r.contains("if env")));
    assert_eq!(w.depends_on, vec!["redis".to_string()]);
    assert_eq!(ir.ingress.len(), 1);
    let ing = &ir.ingress[0];
    assert_eq!(ing.name, "gw");
    assert_eq!(ing.host.as_deref(), Some("mall.example.com"));
    assert_eq!(ing.routes.len(), 1);
    assert_eq!(ing.routes[0].path.as_deref(), Some("/"));
    assert_eq!(ing.routes[0].backend.as_deref(), Some("services.gateway"));
    assert_eq!(ing.routes[0].port.as_deref(), Some("http"));
  }
}
