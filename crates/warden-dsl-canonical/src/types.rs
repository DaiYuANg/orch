use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalApplication {
  pub metadata: CanonicalMetadata,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub workloads: Vec<CanonicalWorkload>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub configs: Vec<CanonicalConfig>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub secrets: Vec<CanonicalSecret>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub volumes: Vec<CanonicalVolume>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub ingresses: Vec<CanonicalIngress>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalMetadata {
  pub name: String,
  pub namespace: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum WorkloadKind {
  Service,
  Worker,
  Job,
  Cron,
  Stateful,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum RuntimeKind {
  Docker,
  Containerd,
  Firecracker,
  Process,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum EndpointProtocol {
  Tcp,
  Udp,
  Http,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Default)]
pub struct CanonicalRun {
  pub image: Option<String>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub env: Vec<CanonicalEnvVar>,
  #[serde(skip_serializing_if = "Option::is_none")]
  pub resources: Option<CanonicalResources>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalWorkloadRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalVolumeRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalConfigRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalSecretRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalEndpointRef {
  pub workload: String,
  pub endpoint: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalEndpoint {
  pub name: String,
  pub port: u16,
  pub protocol: EndpointProtocol,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalWorkload {
  pub name: String,
  pub kind: WorkloadKind,
  pub runtime: RuntimeKind,
  pub run: CanonicalRun,
  pub replicas: Option<u32>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub depends_on: Vec<CanonicalWorkloadRef>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub endpoints: Vec<CanonicalEndpoint>,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub mounts: Vec<CanonicalMount>,
  #[serde(skip_serializing_if = "Option::is_none")]
  pub health: Option<CanonicalHealth>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalIngress {
  pub name: String,
  pub host: String,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub routes: Vec<CanonicalIngressRoute>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalIngressRoute {
  pub path: String,
  pub backend: CanonicalEndpointRef,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalIngressRouteSpec {
  pub id: String,
  pub ingress_name: String,
  pub protocol: String,
  pub host: String,
  pub path_prefix: String,
  pub listen_port: u16,
  pub backend: CanonicalEndpointRef,
  pub dns_enabled: bool,
  pub dns_ttl: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalApplyOutput {
  pub application: String,
  pub namespace: String,
  #[serde(default, skip_serializing_if = "Vec::is_empty")]
  pub ingress_routes: Vec<CanonicalIngressRouteSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalConfig {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalSecret {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalVolume {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalMount {
  pub volume: CanonicalVolumeRef,
  pub target: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalEnvVar {
  pub name: String,
  pub value: CanonicalEnvValue,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalResources {
  pub cpu_millis: Option<u32>,
  pub memory_bytes: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalHttpProbe {
  pub path: String,
  pub endpoint: CanonicalEndpointRef,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CanonicalHealth {
  pub readiness: Option<CanonicalHttpProbe>,
  pub liveness: Option<CanonicalHttpProbe>,
  pub startup: Option<CanonicalHttpProbe>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CanonicalEnvValue {
  String(String),
  ConfigRef(CanonicalConfigRef),
  SecretRef(CanonicalSecretRef),
  EndpointRef(CanonicalEndpointRef),
}

#[derive(Debug, Error)]
pub enum CanonicalLowerError {
  #[error("let {0} must use a string literal for canonical lowering")]
  NonStringLet(String),
  #[error("unsupported workload kind '{0}' in canonical lowering")]
  UnsupportedWorkloadKind(String),
  #[error("unsupported runtime '{0}' in canonical lowering")]
  UnsupportedRuntime(String),
  #[error("unsupported endpoint protocol '{0}' in canonical lowering")]
  UnsupportedEndpointProtocol(String),
  #[error("unsupported env value '{0}' in canonical lowering")]
  InvalidEnvValue(String),
  #[error("invalid cpu resource '{0}' in canonical lowering")]
  InvalidCpuResource(String),
  #[error("invalid memory resource '{0}' in canonical lowering")]
  InvalidMemoryResource(String),
  #[error("invalid replicas expression: {0}")]
  InvalidReplicas(String),
  #[error("unknown let variable '{0}' in interpolation")]
  UnknownInterpolation(String),
  #[error("unclosed interpolation in '{0}'")]
  UnclosedInterpolation(String),
  #[error("{0}")]
  Bind(String),
}
