use crate::RouteRecord;
use serde::{Deserialize, Serialize};
use utoipa::ToSchema;

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct DslApplyRequest {
  pub manifest_yaml: String,
  pub prune: bool,
  pub strict: bool,
  pub concurrency: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct DslApplyResult {
  pub application: String,
  pub namespace: String,
  pub created: Vec<String>,
  pub kept: Vec<String>,
  pub pruned: Vec<String>,
  pub rolled_back: Vec<String>,
  pub warnings: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DslIngressRouteRecord {
  pub route: RouteRecord,
  pub dns_enabled: bool,
  pub dns_ttl: u32,
}
