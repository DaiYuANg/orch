use crate::compile::CompiledManifest;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManifestPlan {
  pub application: String,
  pub namespace: String,
  pub prefix: String,
  pub create: Vec<String>,
  pub keep: Vec<String>,
  pub delete_candidates: Vec<String>,
  pub warnings: Vec<String>,
}

pub fn build_plan(compiled: &CompiledManifest, existing_names: &[String]) -> ManifestPlan {
  let desired = compiled
    .workloads
    .iter()
    .map(|item| item.name.clone())
    .collect::<Vec<_>>();
  let desired_set = desired.iter().cloned().collect::<HashSet<_>>();
  let existing_set = existing_names.iter().cloned().collect::<HashSet<_>>();

  let mut create = desired
    .iter()
    .filter(|name| !existing_set.contains(*name))
    .cloned()
    .collect::<Vec<_>>();
  let mut keep = desired
    .iter()
    .filter(|name| existing_set.contains(*name))
    .cloned()
    .collect::<Vec<_>>();
  let mut delete_candidates = existing_names
    .iter()
    .filter(|name| name.starts_with(&compiled.prefix) && !desired_set.contains(*name))
    .cloned()
    .collect::<Vec<_>>();

  create.sort();
  keep.sort();
  delete_candidates.sort();

  ManifestPlan {
    application: compiled.application.clone(),
    namespace: compiled.namespace.clone(),
    prefix: compiled.prefix.clone(),
    create,
    keep,
    delete_candidates,
    warnings: sorted_unique(compiled.warnings.clone()),
  }
}

fn sorted_unique(mut items: Vec<String>) -> Vec<String> {
  items.sort();
  items.dedup();
  items
}
