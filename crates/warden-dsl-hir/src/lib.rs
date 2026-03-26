//! High-level IR: normalized view of an invocation DSL document after AST lowering.
//!
//! Top-level `services { ... }` and `ingress("name") { ... }` blocks are extracted;
//! `let` bindings are collected in source order.

use serde::{Deserialize, Serialize};
use smol_str::SmolStr;
use thiserror::Error;
use warden_dsl_ast::{DocumentAst, ExprAst, PathAst, StmtAst};

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum HirError {
  #[error("duplicate service name: {0}")]
  DuplicateService(String),
  #[error("duplicate let binding: {0}")]
  DuplicateLet(SmolStr),
  #[error("invalid `services` block: only `val ... = create(...)` entries are allowed")]
  InvalidServicesBody,
  #[error("ingress(...) requires a single string argument")]
  MalformedIngress,
  #[error("unexpected top-level statement (expected let, services {{ }}, or ingress(...))")]
  UnexpectedTopLevel,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HirDocument {
  pub app_name: String,
  pub lets: Vec<HirLet>,
  pub services: Vec<HirService>,
  pub ingress_blocks: Vec<HirIngressBlock>,
  /// Top-level statements that are not yet modeled as dedicated HIR (forward compat).
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub orphan_top_level: Vec<StmtAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HirLet {
  pub name: SmolStr,
  pub expr: ExprAst,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HirService {
  pub name: String,
  pub binding: SmolStr,
  pub body: Vec<StmtAst>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HirIngressBlock {
  pub name: String,
  pub body: Vec<StmtAst>,
}

/// Lower a parsed [`DocumentAst`] into [`HirDocument`].
pub fn lower(doc: &DocumentAst) -> Result<HirDocument, HirError> {
  let app_name = doc.app.name.clone();
  let mut lets = Vec::new();
  let mut let_names = rustc_hash::FxHashSet::<SmolStr>::default();
  let mut services = Vec::new();
  let mut service_names = rustc_hash::FxHashSet::<String>::default();
  let mut ingress_blocks = Vec::new();
  let mut orphan_top_level = Vec::new();

  for stmt in &doc.app.body {
    match stmt {
      StmtAst::Let(l) => {
        if !let_names.insert(l.name.clone()) {
          return Err(HirError::DuplicateLet(l.name.clone()));
        }
        lets.push(HirLet {
          name: l.name.clone(),
          expr: l.value.clone(),
        });
      }
      StmtAst::Block(b) if path_is_single(&b.callee, "services") => {
        for nested in &b.body {
          match nested {
            StmtAst::Create(c) => {
              if !service_names.insert(c.name.clone()) {
                return Err(HirError::DuplicateService(c.name.clone()));
              }
              services.push(HirService {
                name: c.name.clone(),
                binding: c.binding.clone(),
                body: c.body.clone(),
              });
            }
            _ => return Err(HirError::InvalidServicesBody),
          }
        }
      }
      StmtAst::Block(b) if path_is_single(&b.callee, "ingress") => {
        let name = first_string_arg(&b.args).ok_or(HirError::MalformedIngress)?;
        ingress_blocks.push(HirIngressBlock {
          name,
          body: b.body.clone(),
        });
      }
      other => orphan_top_level.push(other.clone()),
    }
  }

  Ok(HirDocument {
    app_name,
    lets,
    services,
    ingress_blocks,
    orphan_top_level,
  })
}

fn path_is_single(path: &PathAst, want: &str) -> bool {
  path.segments.len() == 1 && path.segments[0].as_str() == want
}

fn first_string_arg(args: &[ExprAst]) -> Option<String> {
  match args.first()? {
    ExprAst::String(s) => Some(s.clone()),
    _ => None,
  }
}

/// Service name and dependency edges for semantic analysis (mirrors legacy AST walk).
pub fn service_graph_inputs(hir: &HirDocument) -> (Vec<String>, Vec<(String, String)>) {
  let mut names = Vec::new();
  let mut deps = Vec::new();
  for svc in &hir.services {
    names.push(svc.name.clone());
    collect_dep_edges(&svc.name, &svc.body, &mut deps);
  }
  (names, deps)
}

fn collect_dep_edges(from_service: &str, body: &[StmtAst], deps: &mut Vec<(String, String)>) {
  for stmt in body {
    if let StmtAst::Invoke(invoke) = stmt
      && path_is_single(&invoke.callee, "dependsOn")
    {
      for arg in &invoke.args {
        match arg {
          ExprAst::Identifier(v) => deps.push((from_service.to_string(), v.to_string())),
          ExprAst::Path(path) => {
            if let Some(last) = path.segments.last() {
              deps.push((from_service.to_string(), last.to_string()));
            }
          }
          _ => {}
        }
      }
    }
  }
}

#[cfg(test)]
mod tests {
  use super::*;
  use warden_dsl_parser::parse;

  #[test]
  fn lowers_mall_sample() {
    let raw = r#"
app("mall") {
  let env = "prod"
  services {
    val redis = create("redis") { runtime(container) }
    val gateway = create("gateway") {
      dependsOn(redis)
    }
  }
  ingress("gw") { host("h") }
}
"#;
    let ast = parse(raw).unwrap();
    let hir = lower(&ast).unwrap();
    assert_eq!(hir.app_name, "mall");
    assert_eq!(hir.lets.len(), 1);
    assert_eq!(hir.services.len(), 2);
    assert_eq!(hir.ingress_blocks.len(), 1);
    assert_eq!(hir.ingress_blocks[0].name, "gw");
    let (names, deps) = service_graph_inputs(&hir);
    assert!(names.contains(&"redis".to_string()));
    assert!(deps.contains(&("gateway".to_string(), "redis".to_string())));
  }
}
