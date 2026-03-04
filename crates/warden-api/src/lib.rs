use axum::{
  Json, Router,
  extract::{Path, Request, State},
  http::{StatusCode, header::CONTENT_TYPE},
  middleware::{self, Next},
  response::{IntoResponse, Response},
  routing::{get, post},
};
use utoipa::OpenApi;
use utoipa_swagger_ui::SwaggerUi;
use warden_dns::DnsService;
use warden_registry::RegistryService;
use warden_task::TaskService;
use warden_types::{
  ApiEnvelope, DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
  api_code,
};

#[derive(Clone)]
pub struct ApiState {
  pub registry: RegistryService,
  pub dns: DnsService,
  pub task: TaskService,
}

type ApiErr = (StatusCode, Json<ApiEnvelope<String>>);
type ApiResult<T> = Result<Json<ApiEnvelope<T>>, ApiErr>;

#[derive(OpenApi)]
#[openapi(
  paths(healthz, list_workloads, list_tasks, get_task, deploy_task, stop_task),
  components(schemas(DeployWorkloadRequest, WorkloadSummary, EndpointRecord, RouteRecord, DnsRecord)),
  tags((name = "warden-api", description = "Warden API endpoints"))
)]
struct ApiDoc;

pub fn router(state: ApiState) -> Router {
  let docs = SwaggerUi::new("/swagger-ui").url("/api-docs/openapi.json", ApiDoc::openapi());
  Router::new()
    .merge(docs)
    .route("/healthz", get(healthz))
    .route("/workloads", get(list_workloads))
    .route("/system/endpoints", get(list_endpoints))
    .route("/system/routes", get(list_routes))
    .route("/system/dns/records", get(list_dns_records))
    .route("/tasks", get(list_tasks))
    .route("/tasks/deploy", post(deploy_task))
    .route("/tasks/{id}", get(get_task))
    .route("/tasks/{id}/stop", post(stop_task))
    .layer(middleware::from_fn(error_envelope_middleware))
    .with_state(state)
}

pub fn openapi_json() -> utoipa::openapi::OpenApi {
  ApiDoc::openapi()
}

#[utoipa::path(get, path = "/healthz", responses((status = 200, description = "Health check")))]
async fn healthz() -> Json<ApiEnvelope<String>> {
  Json(ApiEnvelope::ok(String::from("ok")))
}

#[utoipa::path(get, path = "/workloads", responses((status = 200, description = "List workloads")))]
async fn list_workloads(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<WorkloadSummary>>> {
  Json(ApiEnvelope::ok(state.registry.list_workloads().await))
}

async fn list_endpoints(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<EndpointRecord>>> {
  Json(ApiEnvelope::ok(state.registry.list_endpoints().await))
}

async fn list_routes(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<RouteRecord>>> {
  Json(ApiEnvelope::ok(state.registry.list_routes().await))
}

async fn list_dns_records(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<DnsRecord>>> {
  Json(ApiEnvelope::ok(state.dns.list_records().await))
}

#[utoipa::path(get, path = "/tasks", responses((status = 200, description = "List tasks")))]
async fn list_tasks(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<WorkloadSummary>>> {
  Json(ApiEnvelope::ok(state.task.list().await))
}

#[utoipa::path(
  get,
  path = "/tasks/{id}",
  params(("id" = String, Path, description = "Task ID")),
  responses((status = 200, description = "Task found"), (status = 404, description = "Task not found"))
)]
async fn get_task(
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
async fn deploy_task(
  State(state): State<ApiState>,
  Json(req): Json<DeployWorkloadRequest>,
) -> ApiResult<WorkloadSummary> {
  if req.name.trim().is_empty() {
    return Err(bad_request("name is required".to_string()));
  }
  match state.task.deploy(req).await {
    Ok(item) => Ok(Json(ApiEnvelope::ok(item))),
    Err(err) => Err(internal_error(err)),
  }
}

#[utoipa::path(
  post,
  path = "/tasks/{id}/stop",
  params(("id" = String, Path, description = "Task ID")),
  responses((status = 200, description = "Task stopped"), (status = 404, description = "Task not found"), (status = 500, description = "Internal error"))
)]
async fn stop_task(
  Path(id): Path<String>,
  State(state): State<ApiState>,
) -> ApiResult<WorkloadSummary> {
  match state.task.stop(&id).await {
    Ok(Some(item)) => Ok(Json(ApiEnvelope::ok(item))),
    Ok(None) => Err(not_found(format!("task {id} not found"))),
    Err(err) => Err(internal_error(err)),
  }
}

fn bad_request(message: String) -> ApiErr {
  api_error(StatusCode::BAD_REQUEST, api_code::INVALID_ARGUMENT, message)
}

fn not_found(message: String) -> ApiErr {
  api_error(StatusCode::NOT_FOUND, api_code::NOT_FOUND, message)
}

fn internal_error(err: impl std::fmt::Display) -> ApiErr {
  api_error(
    StatusCode::INTERNAL_SERVER_ERROR,
    api_code::INTERNAL,
    err.to_string(),
  )
}

fn api_error(status: StatusCode, code: i32, message: String) -> ApiErr {
  (status, Json(ApiEnvelope::err(code, message)))
}

async fn error_envelope_middleware(req: Request, next: Next) -> Response {
  let response = next.run(req).await;
  let status = response.status();
  if !status.is_client_error() && !status.is_server_error() {
    return response;
  }

  let content_type = response
    .headers()
    .get(CONTENT_TYPE)
    .and_then(|v| v.to_str().ok())
    .unwrap_or_default()
    .to_ascii_lowercase();
  if content_type.contains("application/json") {
    return response;
  }

  (
    status,
    Json(ApiEnvelope::err(
      status_to_api_code(status),
      status
        .canonical_reason()
        .unwrap_or("request failed")
        .to_string(),
    )),
  )
    .into_response()
}

fn status_to_api_code(status: StatusCode) -> i32 {
  match status {
    StatusCode::BAD_REQUEST => api_code::INVALID_ARGUMENT,
    StatusCode::NOT_FOUND => api_code::NOT_FOUND,
    _ => api_code::INTERNAL,
  }
}

#[cfg(test)]
mod tests {
  use super::*;

  #[test]
  fn api_error_code_mapping() {
    assert_eq!(
      status_to_api_code(StatusCode::BAD_REQUEST),
      api_code::INVALID_ARGUMENT
    );
    assert_eq!(
      status_to_api_code(StatusCode::NOT_FOUND),
      api_code::NOT_FOUND
    );
    assert_eq!(
      status_to_api_code(StatusCode::INTERNAL_SERVER_ERROR),
      api_code::INTERNAL
    );
  }

  #[test]
  fn openapi_has_tasks_path() {
    let spec = openapi_json();
    assert!(spec.paths.paths.contains_key("/tasks"));
    assert!(spec.paths.paths.contains_key("/tasks/deploy"));
  }
}
