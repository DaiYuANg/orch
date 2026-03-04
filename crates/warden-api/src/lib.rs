use axum::{
    Json, Router,
    extract::{Path, State},
    http::StatusCode,
    routing::{get, post},
};
use warden_dns::DnsService;
use warden_registry::RegistryService;
use warden_task::TaskService;
use warden_types::{
    ApiEnvelope, DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
};

#[derive(Clone)]
pub struct ApiState {
    pub registry: RegistryService,
    pub dns: DnsService,
    pub task: TaskService,
}

type ApiErr = (StatusCode, Json<ApiEnvelope<String>>);
type ApiResult<T> = Result<Json<ApiEnvelope<T>>, ApiErr>;

pub fn router(state: ApiState) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/workloads", get(list_workloads))
        .route("/system/endpoints", get(list_endpoints))
        .route("/system/routes", get(list_routes))
        .route("/system/dns/records", get(list_dns_records))
        .route("/tasks", get(list_tasks))
        .route("/tasks/deploy", post(deploy_task))
        .route("/tasks/{id}", get(get_task))
        .route("/tasks/{id}/stop", post(stop_task))
        .with_state(state)
}

async fn healthz() -> Json<ApiEnvelope<&'static str>> {
    Json(ApiEnvelope::ok("ok"))
}

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

async fn list_tasks(State(state): State<ApiState>) -> Json<ApiEnvelope<Vec<WorkloadSummary>>> {
    Json(ApiEnvelope::ok(state.task.list().await))
}

async fn get_task(
    Path(id): Path<String>,
    State(state): State<ApiState>,
) -> ApiResult<WorkloadSummary> {
    if let Some(item) = state.task.get(&id).await {
        return Ok(Json(ApiEnvelope::ok(item)));
    }
    Err(not_found(format!("task {id} not found")))
}

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
    (
        StatusCode::BAD_REQUEST,
        Json(ApiEnvelope {
            code: StatusCode::BAD_REQUEST.as_u16() as i32,
            message,
            data: String::new(),
        }),
    )
}

fn not_found(message: String) -> ApiErr {
    (
        StatusCode::NOT_FOUND,
        Json(ApiEnvelope {
            code: StatusCode::NOT_FOUND.as_u16() as i32,
            message,
            data: String::new(),
        }),
    )
}

fn internal_error(err: impl std::fmt::Display) -> ApiErr {
    (
        StatusCode::INTERNAL_SERVER_ERROR,
        Json(ApiEnvelope {
            code: StatusCode::INTERNAL_SERVER_ERROR.as_u16() as i32,
            message: err.to_string(),
            data: String::new(),
        }),
    )
}
