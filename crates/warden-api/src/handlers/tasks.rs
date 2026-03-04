use crate::error::{bad_request, internal_error, not_found};
use crate::{ApiResult, ApiState};
use axum::{
  Json,
  extract::{Path, State},
};
use tracing::{info, warn};
use warden_types::{
  ApiEnvelope, BatchActionResult, DeployWorkloadRequest, FailoverRequest, MigrateWorkloadRequest,
  RebalanceRequest, WorkloadSummary,
};

#[utoipa::path(get, path = "/tasks", responses((status = 200, description = "List tasks")))]
pub(crate) async fn list_tasks(
  State(state): State<ApiState>,
) -> Json<ApiEnvelope<Vec<WorkloadSummary>>> {
  Json(ApiEnvelope::ok(state.task.list().await))
}

#[utoipa::path(
  get,
  path = "/tasks/{id}",
  params(("id" = String, Path, description = "Task ID")),
  responses((status = 200, description = "Task found"), (status = 404, description = "Task not found"))
)]
pub(crate) async fn get_task(
  Path(id): Path<String>,
  State(state): State<ApiState>,
) -> ApiResult<WorkloadSummary> {
  if let Some(item) = state.task.get(&id).await {
    return Ok(Json(ApiEnvelope::ok(item)));
  }
  Err(not_found(format!("task {id} not found")))
}

#[utoipa::path(
  post,
  path = "/tasks/deploy",
  request_body = DeployWorkloadRequest,
  responses((status = 200, description = "Task deployed"), (status = 400, description = "Invalid request"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn deploy_task(
  State(state): State<ApiState>,
  Json(req): Json<DeployWorkloadRequest>,
) -> ApiResult<WorkloadSummary> {
  if req.name.trim().is_empty() {
    return Err(bad_request(String::from("name is required")));
  }
  info!(
    target: "warden::api::tasks",
    name = %req.name,
    runtime = %req.runtime,
    "deploy task request"
  );
  match state.task.deploy(req).await {
    Ok(item) => Ok(Json(ApiEnvelope::ok(item))),
    Err(err) => {
      warn!(target: "warden::api::tasks", error = %err, "deploy task failed");
      Err(internal_error(err))
    }
  }
}

#[utoipa::path(
  post,
  path = "/tasks/{id}/stop",
  params(("id" = String, Path, description = "Task ID")),
  responses((status = 200, description = "Task stopped"), (status = 404, description = "Task not found"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn stop_task(
  Path(id): Path<String>,
  State(state): State<ApiState>,
) -> ApiResult<WorkloadSummary> {
  info!(target: "warden::api::tasks", workload_id = %id, "stop task request");
  match state.task.stop(&id).await {
    Ok(Some(item)) => Ok(Json(ApiEnvelope::ok(item))),
    Ok(None) => Err(not_found(format!("task {id} not found"))),
    Err(err) => {
      warn!(
        target: "warden::api::tasks",
        workload_id = %id,
        error = %err,
        "stop task failed"
      );
      Err(internal_error(err))
    }
  }
}

#[utoipa::path(
  post,
  path = "/tasks/{id}/migrate",
  request_body = MigrateWorkloadRequest,
  params(("id" = String, Path, description = "Task ID")),
  responses((status = 200, description = "Task migrated"), (status = 404, description = "Task not found"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn migrate_task(
  Path(id): Path<String>,
  State(state): State<ApiState>,
  Json(req): Json<MigrateWorkloadRequest>,
) -> ApiResult<WorkloadSummary> {
  info!(
    target: "warden::api::tasks",
    workload_id = %id,
    target_node = %req.target_node,
    "migrate task request"
  );
  match state.task.migrate(&id, &req).await {
    Ok(Some(item)) => Ok(Json(ApiEnvelope::ok(item))),
    Ok(None) => Err(not_found(format!("task {id} not found"))),
    Err(err) => {
      warn!(
        target: "warden::api::tasks",
        workload_id = %id,
        error = %err,
        "migrate task failed"
      );
      Err(internal_error(err))
    }
  }
}

#[utoipa::path(
  post,
  path = "/tasks/failover",
  request_body = FailoverRequest,
  responses((status = 200, description = "Failover completed"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn failover_tasks(
  State(state): State<ApiState>,
  Json(req): Json<FailoverRequest>,
) -> ApiResult<BatchActionResult> {
  info!(
    target: "warden::api::tasks",
    failed_node = %req.failed_node,
    target_node = ?req.target_node,
    "failover request"
  );
  match state.task.failover(&req).await {
    Ok(result) => Ok(Json(ApiEnvelope::ok(result))),
    Err(err) => {
      warn!(target: "warden::api::tasks", error = %err, "failover request failed");
      Err(internal_error(err))
    }
  }
}

#[utoipa::path(
  post,
  path = "/tasks/rebalance",
  request_body = RebalanceRequest,
  responses((status = 200, description = "Rebalance completed"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn rebalance_tasks(
  State(state): State<ApiState>,
  Json(req): Json<RebalanceRequest>,
) -> ApiResult<BatchActionResult> {
  info!(
    target: "warden::api::tasks",
    max_migrations = req.max_migrations,
    "rebalance request"
  );
  match state.task.rebalance(&req).await {
    Ok(result) => Ok(Json(ApiEnvelope::ok(result))),
    Err(err) => {
      warn!(target: "warden::api::tasks", error = %err, "rebalance request failed");
      Err(internal_error(err))
    }
  }
}
