use crate::ApiState;
use axum::{Json, extract::State};
use warden_types::{ApiEnvelope, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary};

#[utoipa::path(get, path = "/healthz", responses((status = 200, description = "Health check")))]
pub(crate) async fn healthz() -> Json<ApiEnvelope<String>> {
  Json(ApiEnvelope::ok(String::from("ok")))
}

#[utoipa::path(get, path = "/workloads", responses((status = 200, description = "List workloads")))]
pub(crate) async fn list_workloads(
  State(state): State<ApiState>,
) -> Json<ApiEnvelope<Vec<WorkloadSummary>>> {
  Json(ApiEnvelope::ok(state.registry.list_workloads().await))
}

#[utoipa::path(
  get,
  path = "/system/endpoints",
  responses((status = 200, description = "List endpoint records"))
)]
pub(crate) async fn list_endpoints(
  State(state): State<ApiState>,
) -> Json<ApiEnvelope<Vec<EndpointRecord>>> {
  Json(ApiEnvelope::ok(state.registry.list_endpoints().await))
}

#[utoipa::path(get, path = "/system/routes", responses((status = 200, description = "List route records")))]
pub(crate) async fn list_routes(
  State(state): State<ApiState>,
) -> Json<ApiEnvelope<Vec<RouteRecord>>> {
  Json(ApiEnvelope::ok(state.registry.list_routes().await))
}

#[utoipa::path(
  get,
  path = "/system/dns/records",
  responses((status = 200, description = "List DNS records"))
)]
pub(crate) async fn list_dns_records(
  State(state): State<ApiState>,
) -> Json<ApiEnvelope<Vec<DnsRecord>>> {
  Json(ApiEnvelope::ok(state.dns.list_records().await))
}
