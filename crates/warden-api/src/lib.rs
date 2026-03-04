mod doc;
mod error;
mod handlers;

use axum::{
  Json, Router,
  middleware::{self},
  routing::{get, post},
};
use warden_dns::DnsService;
use warden_raft::RaftService;
use warden_registry::RegistryService;
use warden_task::TaskService;
use warden_types::ApiEnvelope;

#[derive(Clone)]
pub struct ApiState {
  pub registry: RegistryService,
  pub dns: DnsService,
  pub task: TaskService,
  pub raft: RaftService,
}

pub(crate) type ApiErr = (axum::http::StatusCode, Json<ApiEnvelope<String>>);
pub(crate) type ApiResult<T> = Result<Json<ApiEnvelope<T>>, ApiErr>;

pub fn router(state: ApiState) -> Router {
  let docs: Router<ApiState> = doc::swagger_router().with_state(());
  Router::<ApiState>::new()
    .merge(docs)
    .route("/healthz", get(handlers::read::healthz))
    .route("/workloads", get(handlers::read::list_workloads))
    .route("/system/endpoints", get(handlers::read::list_endpoints))
    .route("/system/routes", get(handlers::read::list_routes))
    .route("/system/dns/records", get(handlers::read::list_dns_records))
    .route("/system/info", get(handlers::system::system_info))
    .route("/system/cluster", get(handlers::system::cluster_info))
    .route("/system/cpu", get(handlers::system::cpu_info))
    .route("/system/mem", get(handlers::system::mem_info))
    .route("/system/disk", get(handlers::system::disk_info))
    .route("/system/runtime", get(handlers::system::runtime_info))
    .route("/tasks", get(handlers::tasks::list_tasks))
    .route("/tasks/deploy", post(handlers::tasks::deploy_task))
    .route("/tasks/failover", post(handlers::tasks::failover_tasks))
    .route("/tasks/rebalance", post(handlers::tasks::rebalance_tasks))
    .route("/tasks/{id}", get(handlers::tasks::get_task))
    .route("/tasks/{id}/stop", post(handlers::tasks::stop_task))
    .route("/tasks/{id}/migrate", post(handlers::tasks::migrate_task))
    .layer(middleware::from_fn(error::error_envelope_middleware))
    .with_state(state)
}

pub fn openapi_json() -> utoipa::openapi::OpenApi {
  doc::openapi_json()
}
