use crate::error::{internal_error, not_found};
use crate::{ApiResult, ApiState};
use axum::{
  Json,
  extract::{Path, Query, State},
};
use tracing::{info, warn};
use warden_types::{ApiEnvelope, TaskLogsResponse};

#[derive(Debug, serde::Deserialize)]
pub(crate) struct TaskLogsQuery {
  tail: Option<usize>,
}

#[utoipa::path(
  get,
  path = "/tasks/{id}/logs",
  params(
    ("id" = String, Path, description = "Task ID"),
    ("tail" = Option<usize>, Query, description = "Tail line count, default 200")
  ),
  responses((status = 200, description = "Task logs"), (status = 404, description = "Task not found"), (status = 500, description = "Internal error"))
)]
pub(crate) async fn task_logs(
  Path(id): Path<String>,
  Query(query): Query<TaskLogsQuery>,
  State(state): State<ApiState>,
) -> ApiResult<TaskLogsResponse> {
  let tail = query.tail.unwrap_or(200).max(1);
  info!(
    target: "warden::api::tasks",
    workload_id = %id,
    tail,
    "task logs request"
  );
  match state.task.logs(&id, tail).await {
    Ok(Some(item)) => Ok(Json(ApiEnvelope::ok(item))),
    Ok(None) => Err(not_found(format!("task {id} not found"))),
    Err(err) => {
      warn!(
        target: "warden::api::tasks",
        workload_id = %id,
        tail,
        error = %err,
        "task logs request failed"
      );
      Err(internal_error(err))
    }
  }
}
