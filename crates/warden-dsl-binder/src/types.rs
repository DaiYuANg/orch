use serde::{Deserialize, Serialize};
use thiserror::Error;
use warden_dsl_ast::ExprAst;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundDocument {
  pub app_name: String,
  pub lets: Vec<BoundLet>,
  pub volumes: Vec<BoundVolume>,
  pub configs: Vec<BoundConfig>,
  pub secrets: Vec<BoundSecret>,
  pub workloads: Vec<BoundWorkload>,
  pub ingresses: Vec<BoundIngress>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundLet {
  pub name: String,
  pub expr: ExprAst,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundWorkloadRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundVolumeRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundConfigRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundSecretRef {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundEndpointRef {
  pub workload: BoundWorkloadRef,
  pub endpoint: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundVolume {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundConfig {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundSecret {
  pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundMount {
  pub volume: BoundVolumeRef,
  pub target: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundEnvVar {
  pub name: String,
  pub value: BoundEnvValue,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundResources {
  pub cpu: Option<String>,
  pub memory: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundHttpProbe {
  pub path: String,
  pub endpoint: BoundEndpointRef,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BoundHealth {
  pub readiness: Option<BoundHttpProbe>,
  pub liveness: Option<BoundHttpProbe>,
  pub startup: Option<BoundHttpProbe>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum BoundEnvValue {
  String(String),
  ConfigRef(BoundConfigRef),
  SecretRef(BoundSecretRef),
  EndpointRef(BoundEndpointRef),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundWorkload {
  pub binding: String,
  pub name: String,
  pub kind: Option<String>,
  pub runtime: Option<String>,
  pub image: Option<String>,
  pub endpoint_name: Option<String>,
  pub endpoint_protocol: Option<String>,
  pub service_port: Option<u16>,
  pub replicas: Option<String>,
  pub depends_on: Vec<BoundWorkloadRef>,
  pub mounts: Vec<BoundMount>,
  pub env: Vec<BoundEnvVar>,
  pub resources: Option<BoundResources>,
  pub health: Option<BoundHealth>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundIngress {
  pub name: String,
  pub host: Option<String>,
  pub routes: Vec<BoundIngressRoute>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct BoundIngressRoute {
  pub path: Option<String>,
  pub backend: Option<BoundEndpointRef>,
}

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum BindError {
  #[error("duplicate workload binding: {0}")]
  DuplicateBinding(String),
  #[error("unknown workload reference '{raw}' in {context}")]
  UnknownWorkloadRef { raw: String, context: String },
  #[error("invalid workload reference '{raw}' in {context}")]
  InvalidWorkloadRef { raw: String, context: String },
  #[error(
    "backend(...) requires a workload or endpoint reference in ingress {ingress} route {path}"
  )]
  InvalidBackendRef { ingress: String, path: String },
  #[error("invalid container port '{raw}' for workload {workload}")]
  InvalidContainerPort { workload: String, raw: String },
  #[error("invalid endpoint port '{raw}' for workload {workload}")]
  InvalidEndpointPort { workload: String, raw: String },
  #[error("unknown volume reference '{raw}' in {context}")]
  UnknownVolumeRef { raw: String, context: String },
  #[error("invalid volume reference '{raw}' in {context}")]
  InvalidVolumeRef { raw: String, context: String },
  #[error("unknown config reference '{raw}' in {context}")]
  UnknownConfigRef { raw: String, context: String },
  #[error("invalid config reference '{raw}' in {context}")]
  InvalidConfigRef { raw: String, context: String },
  #[error("unknown secret reference '{raw}' in {context}")]
  UnknownSecretRef { raw: String, context: String },
  #[error("invalid secret reference '{raw}' in {context}")]
  InvalidSecretRef { raw: String, context: String },
  #[error("mount(...) requires a string target in workload {workload}")]
  InvalidMountTarget { workload: String },
  #[error("set(...) requires a string key and value in workload {workload}")]
  InvalidEnvSet { workload: String },
  #[error("invalid endpoint reference '{raw}' in {context}")]
  InvalidEndpointRef { raw: String, context: String },
  #[error("unknown endpoint reference '{raw}' in {context}")]
  UnknownEndpointRef { raw: String, context: String },
  #[error("unsupported env value '{raw}' in workload {workload}")]
  InvalidEnvValue { workload: String, raw: String },
}
