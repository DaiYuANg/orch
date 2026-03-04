use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use utoipa::ToSchema;

pub mod api_code {
  pub const OK: i32 = 0;
  pub const INVALID_ARGUMENT: i32 = 1001;
  pub const NOT_FOUND: i32 = 1004;
  pub const INTERNAL: i32 = 2000;
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct ApiEnvelope<T> {
  pub code: i32,
  pub message: String,
  pub data: T,
}

impl<T> ApiEnvelope<T> {
  pub fn ok(data: T) -> Self {
    Self {
      code: api_code::OK,
      message: String::from("ok"),
      data,
    }
  }
}

impl ApiEnvelope<String> {
  pub fn err(code: i32, message: impl Into<String>) -> Self {
    Self {
      code,
      message: message.into(),
      data: String::new(),
    }
  }
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct WorkloadSummary {
  pub id: String,
  pub name: String,
  pub runtime: String,
  pub status: String,
  pub node_id: String,
  pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct EndpointRecord {
  pub workload_id: String,
  pub node_id: String,
  pub protocol: String,
  pub address: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct RouteRecord {
  pub id: String,
  pub protocol: String,
  pub host: String,
  pub path_prefix: String,
  pub listen_port: u16,
  pub backend: String,
  pub enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct DnsRecord {
  pub domain: String,
  pub values: Vec<String>,
  pub ttl: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct DeployWorkloadRequest {
  pub name: String,
  pub runtime: String,
  pub image: Option<String>,
  pub host: Option<String>,
  pub path_prefix: Option<String>,
  pub service_port: Option<u16>,
  pub ingress_port: Option<u16>,
  pub backend: Option<String>,
}
