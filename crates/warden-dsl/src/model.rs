use anyhow::{Context, bail};
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::path::Path;

const API_VERSION: &str = "warden.io/v1alpha1";
const KIND: &str = "Application";

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ApplicationManifest {
  pub api_version: String,
  pub kind: String,
  pub metadata: Metadata,
  pub spec: Spec,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Metadata {
  pub name: String,
  #[serde(default = "default_namespace")]
  pub namespace: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Spec {
  #[serde(default)]
  pub workloads: Vec<WorkloadSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkloadSpec {
  pub name: String,
  #[serde(default = "default_runtime")]
  pub runtime: String,
  pub image: Option<String>,
  pub process: Option<ProcessSpec>,
  pub firecracker: Option<FirecrackerSpec>,
  pub service: Option<ServiceSpec>,
  pub ingress: Option<IngressSpec>,
  pub dns: Option<DnsSpec>,
  pub scheduling: Option<SchedulingSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessSpec {
  pub command: Option<String>,
  #[serde(default)]
  pub args: Vec<String>,
  #[serde(default)]
  pub env: std::collections::BTreeMap<String, String>,
  pub cwd: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FirecrackerSpec {
  pub config: Option<String>,
  pub kernel_image: Option<String>,
  pub rootfs: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceSpec {
  pub port: Option<u16>,
  pub backend: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IngressSpec {
  pub enabled: Option<bool>,
  pub host: Option<String>,
  pub path: Option<String>,
  pub listen_port: Option<u16>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DnsSpec {
  pub enabled: Option<bool>,
  pub ttl: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SchedulingSpec {
  pub stateful: Option<bool>,
  pub allow_leader: Option<bool>,
  #[serde(default)]
  pub preferred_nodes: Vec<String>,
}

impl ApplicationManifest {
  pub fn validate(&self) -> anyhow::Result<()> {
    if self.api_version.trim() != API_VERSION {
      bail!("apiVersion must be {}", API_VERSION);
    }
    if self.kind.trim() != KIND {
      bail!("kind must be {}", KIND);
    }
    let app = self.metadata.name.trim();
    if app.is_empty() {
      bail!("metadata.name is required");
    }
    if self.spec.workloads.is_empty() {
      bail!("spec.workloads must not be empty");
    }

    let mut names = HashSet::new();
    for workload in &self.spec.workloads {
      let name = workload.name.trim();
      if !is_name_token(name) {
        bail!("invalid workload name: {}", workload.name);
      }
      if !names.insert(name.to_string()) {
        bail!("duplicate workload name: {}", workload.name);
      }
      if workload.runtime.trim().is_empty() {
        bail!("runtime is required for workload {}", workload.name);
      }
      if workload.runtime.trim() == "process" {
        let process = workload
          .process
          .as_ref()
          .with_context(|| format!("process block required for workload {}", workload.name))?;
        let has_command = non_empty(process.command.as_deref()).is_some();
        if !has_command {
          bail!("process.command is required for workload {}", workload.name);
        }
      }
      if let Some(ingress) = workload.ingress.as_ref() {
        if let Some(path) = ingress.path.as_deref() {
          if !path.trim().starts_with('/') {
            bail!(
              "ingress.path must start with / for workload {}",
              workload.name
            );
          }
        }
      }
      if workload.runtime.trim() == "firecracker" {
        let firecracker = workload
          .firecracker
          .as_ref()
          .with_context(|| format!("firecracker block required for workload {}", workload.name))?;
        let has_config = non_empty(firecracker.config.as_deref()).is_some();
        let has_kernel = non_empty(firecracker.kernel_image.as_deref()).is_some();
        let has_rootfs = non_empty(firecracker.rootfs.as_deref()).is_some();
        if !has_config && !(has_kernel && has_rootfs) {
          bail!(
            "firecracker workload {} needs firecracker.config or kernelImage+rootfs",
            workload.name
          );
        }
      }
    }
    Ok(())
  }

  pub fn namespace(&self) -> &str {
    self.metadata.namespace.trim()
  }

  pub fn application(&self) -> &str {
    self.metadata.name.trim()
  }
}

pub fn load_manifest(path: &Path) -> anyhow::Result<ApplicationManifest> {
  let raw = std::fs::read_to_string(path)
    .with_context(|| format!("read manifest file {}", path.display()))?;
  parse_manifest_yaml(&raw).with_context(|| format!("decode yaml {}", path.display()))
}

pub fn parse_manifest_yaml(raw: &str) -> anyhow::Result<ApplicationManifest> {
  let manifest: ApplicationManifest = serde_yaml::from_str(raw).context("decode manifest yaml")?;
  manifest.validate()?;
  Ok(manifest)
}

fn default_namespace() -> String {
  String::from("default")
}

fn default_runtime() -> String {
  String::from("docker")
}

fn is_name_token(value: &str) -> bool {
  !value.is_empty()
    && value
      .chars()
      .all(|ch| ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' || ch == '.')
}

pub(crate) fn non_empty(value: Option<&str>) -> Option<String> {
  value
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .map(ToString::to_string)
}
