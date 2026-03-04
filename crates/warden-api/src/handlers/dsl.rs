use crate::error::{bad_request, internal_error};
use crate::{ApiResult, ApiState};
use axum::{Json, extract::State};
use futures_util::stream::{self, StreamExt};
use std::collections::HashMap;
use warden_dsl::{build_plan, compile_manifest, parse_manifest_yaml};
use warden_types::dsl::{DslApplyRequest, DslApplyResult};
use warden_types::{ApiEnvelope, DeployWorkloadRequest};

#[utoipa::path(
  post,
  path = "/dsl/apply",
  request_body = DslApplyRequest,
  responses((status = 200, description = "DSL apply completed"), (status = 400, description = "Invalid manifest"), (status = 500, description = "Apply failed"))
)]
pub(crate) async fn apply_dsl(
  State(state): State<ApiState>,
  Json(req): Json<DslApplyRequest>,
) -> ApiResult<DslApplyResult> {
  let manifest =
    parse_manifest_yaml(&req.manifest_yaml).map_err(|err| bad_request(err.to_string()))?;
  let compiled = compile_manifest(&manifest).map_err(|err| bad_request(err.to_string()))?;
  if req.strict && !compiled.warnings.is_empty() {
    return Err(bad_request(format!(
      "dsl strict mode rejected {} warnings",
      compiled.warnings.len()
    )));
  }

  let existing = state.registry.list_workloads().await;
  let names = existing
    .iter()
    .map(|item| item.name.clone())
    .collect::<Vec<_>>();
  let plan = build_plan(&compiled, &names);
  if req.strict && !plan.warnings.is_empty() {
    return Err(bad_request(format!(
      "dsl strict mode rejected {} warnings",
      plan.warnings.len()
    )));
  }

  let request_map = compiled
    .workloads
    .iter()
    .map(|item| (item.name.clone(), item.request.clone()))
    .collect::<HashMap<String, DeployWorkloadRequest>>();
  let id_by_name = existing
    .iter()
    .map(|item| (item.name.clone(), item.id.clone()))
    .collect::<HashMap<String, String>>();
  let concurrency = req.concurrency.max(1).min(32);

  let create_jobs = plan
    .create
    .iter()
    .filter_map(|name| {
      request_map
        .get(name)
        .cloned()
        .map(|deploy| (name.clone(), deploy))
    })
    .collect::<Vec<_>>();
  let (created, create_errors) = deploy_batch(state.clone(), create_jobs, concurrency).await;
  if !create_errors.is_empty() {
    let rollback_map = state
      .registry
      .list_workloads()
      .await
      .into_iter()
      .map(|item| (item.name, item.id))
      .collect::<HashMap<String, String>>();
    let rolled_back = rollback_batch(state.clone(), created, rollback_map, concurrency).await;
    let error = format!(
      "dsl apply create failed: {} ; rolled_back={}",
      create_errors.join(" | "),
      rolled_back.join(",")
    );
    return Err(internal_error(error));
  }

  let mut pruned = Vec::new();
  if req.prune {
    let prune_jobs = plan
      .delete_candidates
      .iter()
      .filter_map(|name| id_by_name.get(name).cloned().map(|id| (name.clone(), id)))
      .collect::<Vec<_>>();
    let (deleted, delete_errors) = stop_batch(state.clone(), prune_jobs, concurrency).await;
    pruned = deleted;
    if !delete_errors.is_empty() {
      let error = format!("dsl apply prune failed: {}", delete_errors.join(" | "));
      return Err(internal_error(error));
    }
  }

  let result = DslApplyResult {
    application: compiled.application,
    namespace: compiled.namespace,
    created,
    kept: plan.keep,
    pruned,
    rolled_back: Vec::new(),
    warnings: plan.warnings,
  };
  Ok(Json(ApiEnvelope::ok(result)))
}

async fn deploy_batch(
  state: ApiState,
  jobs: Vec<(String, DeployWorkloadRequest)>,
  concurrency: usize,
) -> (Vec<String>, Vec<String>) {
  let rows = stream::iter(jobs.into_iter().map(|(name, deploy)| {
    let state = state.clone();
    async move {
      match state.task.deploy(deploy).await {
        Ok(_) => Ok(name),
        Err(err) => Err(format!("{name}: {err}")),
      }
    }
  }))
  .buffer_unordered(concurrency)
  .collect::<Vec<_>>()
  .await;
  partition_batch(rows)
}

async fn rollback_batch(
  state: ApiState,
  created: Vec<String>,
  id_by_name: HashMap<String, String>,
  concurrency: usize,
) -> Vec<String> {
  let jobs = created
    .into_iter()
    .map(|name| {
      let state = state.clone();
      let id_by_name = id_by_name.clone();
      async move {
        if let Some(id) = id_by_name.get(&name).cloned() {
          let _ = state.task.stop(&id).await;
          return Some(name);
        }
        None
      }
    })
    .collect::<Vec<_>>();
  stream::iter(jobs)
    .buffer_unordered(concurrency)
    .filter_map(|value| async move { value })
    .collect::<Vec<_>>()
    .await
}

async fn stop_batch(
  state: ApiState,
  jobs: Vec<(String, String)>,
  concurrency: usize,
) -> (Vec<String>, Vec<String>) {
  let rows = stream::iter(jobs.into_iter().map(|(name, id)| {
    let state = state.clone();
    async move {
      match state.task.stop(&id).await {
        Ok(Some(_)) => Ok(name),
        Ok(None) => Ok(name),
        Err(err) => Err(format!("{name}: {err}")),
      }
    }
  }))
  .buffer_unordered(concurrency)
  .collect::<Vec<_>>()
  .await;
  partition_batch(rows)
}

fn partition_batch(rows: Vec<Result<String, String>>) -> (Vec<String>, Vec<String>) {
  let mut ok = Vec::new();
  let mut err = Vec::new();
  for row in rows {
    match row {
      Ok(name) => ok.push(name),
      Err(item) => err.push(item),
    }
  }
  ok.sort();
  err.sort();
  (ok, err)
}
