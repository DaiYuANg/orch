use petgraph::algo::toposort;
use petgraph::graph::{DiGraph, NodeIndex};
use std::collections::HashMap;
use thiserror::Error;
use warden_dsl_ast::DocumentAst;
use warden_dsl_hir::{HirDocument, HirError, lower, service_graph_inputs};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SemanticModel {
  pub creation_order: Vec<String>,
}

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum SemanticError {
  #[error("duplicate service name: {0}")]
  DuplicateService(String),
  #[error("dependency cycle detected among services")]
  DependencyCycle,
  #[error(transparent)]
  Hir(#[from] HirError),
}

/// Semantic analysis on a lowered [`HirDocument`] (service DAG, creation order).
pub fn analyze_hir(hir: &HirDocument) -> Result<SemanticModel, SemanticError> {
  let mut graph: DiGraph<String, ()> = DiGraph::new();
  let mut nodes: HashMap<String, NodeIndex> = HashMap::new();
  let (names, deps) = service_graph_inputs(hir);

  for name in names {
    if nodes.contains_key(&name) {
      return Err(SemanticError::DuplicateService(name));
    }
    let idx = graph.add_node(name.clone());
    nodes.insert(name, idx);
  }

  for (from, to) in deps {
    if let (Some(from_idx), Some(to_idx)) = (nodes.get(&from), nodes.get(&to)) {
      graph.add_edge(*to_idx, *from_idx, ());
    }
  }

  let sorted = toposort(&graph, None).map_err(|_| SemanticError::DependencyCycle)?;
  let creation_order = sorted
    .iter()
    .filter_map(|idx| graph.node_weight(*idx).cloned())
    .collect::<Vec<_>>();
  Ok(SemanticModel { creation_order })
}

/// Parse-time document analysis: AST → HIR → semantics.
pub fn analyze(doc: &DocumentAst) -> Result<SemanticModel, SemanticError> {
  let hir = lower(doc)?;
  analyze_hir(&hir)
}
