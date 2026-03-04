use crate::handlers;
use axum::Router;
use utoipa::OpenApi;
use utoipa_swagger_ui::SwaggerUi;
use warden_types::dsl::{DslApplyRequest, DslApplyResult};
use warden_types::{
  BatchActionResult, ClusterInfo, ClusterNodeInfo, CpuInfo, DeployWorkloadRequest, DiskInfo,
  DnsRecord, EndpointRecord, FailoverRequest, MemInfo, MigrateWorkloadRequest, RebalanceRequest,
  RouteRecord, RuntimeInfo, RuntimeManagedEntry, SystemInfo, TaskLogsResponse, WorkloadSummary,
};

#[derive(OpenApi)]
#[openapi(
  paths(
    handlers::read::healthz,
    handlers::read::list_workloads,
    handlers::tasks::list_tasks,
    handlers::tasks::get_task,
    handlers::tasks::deploy_task,
    handlers::dsl::apply_dsl,
    handlers::task_logs::task_logs,
    handlers::tasks::stop_task,
    handlers::tasks::migrate_task,
    handlers::tasks::failover_tasks,
    handlers::tasks::rebalance_tasks,
    handlers::system::system_info,
    handlers::system::cluster_info,
    handlers::system::cpu_info,
    handlers::system::mem_info,
    handlers::system::disk_info,
    handlers::system::runtime_info
  ),
  components(
    schemas(
      DeployWorkloadRequest,
      MigrateWorkloadRequest,
      FailoverRequest,
      RebalanceRequest,
      DslApplyRequest,
      DslApplyResult,
      BatchActionResult,
      TaskLogsResponse,
      WorkloadSummary,
      EndpointRecord,
      RouteRecord,
      DnsRecord,
      SystemInfo,
      CpuInfo,
      MemInfo,
      DiskInfo,
      RuntimeInfo,
      RuntimeManagedEntry,
      ClusterInfo,
      ClusterNodeInfo
    )
  ),
  tags((name = "warden-api", description = "Warden API endpoints"))
)]
struct ApiDoc;

pub(crate) fn swagger_router() -> Router {
  SwaggerUi::new("/swagger-ui")
    .url("/api-docs/openapi.json", ApiDoc::openapi())
    .into()
}

pub(crate) fn openapi_json() -> utoipa::openapi::OpenApi {
  ApiDoc::openapi()
}
