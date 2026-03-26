use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use utoipa::ToSchema;

pub mod dsl;

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
  #[serde(default = "default_endpoint_name")]
  pub endpoint_name: String,
  pub protocol: String,
  pub address: String,
  #[serde(default = "default_true")]
  pub healthy: bool,
  #[serde(default = "default_true")]
  pub ready: bool,
  #[serde(default = "default_updated_at")]
  pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct RouteRecord {
  pub id: String,
  pub protocol: String,
  pub host: String,
  pub path_prefix: String,
  pub listen_port: u16,
  pub backend: String,
  #[serde(default)]
  pub backend_workload_id: Option<String>,
  #[serde(default)]
  pub backend_endpoint_name: Option<String>,
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
  pub firecracker_config: Option<String>,
  pub firecracker_kernel_image: Option<String>,
  pub firecracker_rootfs: Option<String>,
  pub host: Option<String>,
  pub path_prefix: Option<String>,
  pub service_port: Option<u16>,
  pub ingress_port: Option<u16>,
  pub backend: Option<String>,
  pub process_command: Option<String>,
  #[serde(default)]
  pub process_args: Vec<String>,
  #[serde(default)]
  pub process_env: BTreeMap<String, String>,
  pub process_cwd: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct MigrateWorkloadRequest {
  pub target_node: String,
  pub force_stateful: bool,
  pub max_unavailable: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct FailoverRequest {
  pub failed_node: String,
  pub target_node: Option<String>,
  pub force_stateful: bool,
  pub max_unavailable: u32,
  pub max_migrations: Option<usize>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct RebalanceRequest {
  pub max_migrations: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct BatchActionResult {
  pub moved: Vec<String>,
  pub skipped: Vec<String>,
  pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct TaskLogsResponse {
  pub workload_id: String,
  pub runtime: String,
  pub tail: usize,
  pub lines: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct SystemInfo {
  pub hostname: String,
  pub uptime: u64,
  pub os: String,
  pub platform: String,
  pub kernel_version: String,
  pub kernel_arch: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct CpuInfo {
  pub model_name: String,
  pub cores: usize,
  pub usage_percent: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct MemInfo {
  pub total: u64,
  pub used: u64,
  pub free: u64,
  pub used_percent: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct DiskInfo {
  pub device: String,
  pub mountpoint: String,
  pub total: u64,
  pub used: u64,
  pub free: u64,
  pub used_percent: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct RuntimeManagedEntry {
  pub workload_id: String,
  pub runtime: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct RuntimeInfo {
  pub providers: Vec<String>,
  pub managed: Vec<RuntimeManagedEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct ClusterNodeInfo {
  pub node_id: String,
  pub workloads: usize,
  pub endpoints: usize,
  pub runtimes: Vec<String>,
  pub healthy: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, ToSchema)]
pub struct ClusterInfo {
  pub raft_enabled: bool,
  pub raft_is_leader: bool,
  pub raft_applied_index: u64,
  pub raft_node_id: u64,
  pub raft_bind_addr: String,
  pub leader_node: Option<String>,
  pub total_nodes: usize,
  pub total_workloads: usize,
  pub nodes: Vec<ClusterNodeInfo>,
}

fn default_endpoint_name() -> String {
  String::from("default")
}

fn default_true() -> bool {
  true
}

fn default_updated_at() -> DateTime<Utc> {
  Utc::now()
}
