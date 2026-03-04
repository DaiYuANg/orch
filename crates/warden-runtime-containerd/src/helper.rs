use anyhow::{Context, anyhow};
use containerd_client::services::v1::{DeleteContainerRequest, DeleteTaskRequest, KillRequest};
use containerd_client::tonic::Request;
use containerd_client::{Client, tonic, with_namespace};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct ContainerdRuntimeConfig {
  pub endpoint: String,
  pub namespace: String,
  pub runtime_name: String,
  pub snapshotter: String,
  pub rootfs: Option<String>,
  pub spec_path: Option<String>,
}

impl ContainerdRuntimeConfig {
  pub fn from_env() -> Self {
    Self {
      endpoint: read_env("WARDEN_CONTAINERD_ENDPOINT")
        .unwrap_or_else(|| String::from(default_endpoint())),
      namespace: read_env("WARDEN_CONTAINERD_NAMESPACE").unwrap_or_else(|| String::from("default")),
      runtime_name: read_env("WARDEN_CONTAINERD_RUNTIME")
        .unwrap_or_else(|| String::from("io.containerd.runc.v2")),
      snapshotter: read_env("WARDEN_CONTAINERD_SNAPSHOTTER")
        .unwrap_or_else(|| String::from("overlayfs")),
      rootfs: read_env("WARDEN_CONTAINERD_ROOTFS"),
      spec_path: read_env("WARDEN_CONTAINERD_SPEC_PATH"),
    }
  }
}

pub(crate) async fn check_connection(cfg: &ContainerdRuntimeConfig) -> anyhow::Result<()> {
  let client = connect(cfg).await?;
  let _ = client
    .version()
    .version(())
    .await
    .context("query version")?;
  Ok(())
}

pub(crate) async fn remove_existing(
  cfg: &ContainerdRuntimeConfig,
  name: &str,
) -> anyhow::Result<()> {
  let client = connect(cfg).await?;
  let ns = cfg.namespace.as_str();

  let mut tasks = client.tasks();
  let kill_req = KillRequest {
    container_id: name.to_string(),
    exec_id: String::new(),
    signal: 9,
    all: true,
  };
  let kill_req = with_namespace!(kill_req, ns);
  ignore_not_found(tasks.kill(kill_req).await)?;

  let delete_task_req = DeleteTaskRequest {
    container_id: name.to_string(),
  };
  let delete_task_req = with_namespace!(delete_task_req, ns);
  ignore_not_found(tasks.delete(delete_task_req).await)?;

  let mut containers = client.containers();
  let delete_container_req = DeleteContainerRequest {
    id: name.to_string(),
  };
  let delete_container_req = with_namespace!(delete_container_req, ns);
  ignore_not_found(containers.delete(delete_container_req).await)?;
  Ok(())
}

pub(crate) async fn connect(cfg: &ContainerdRuntimeConfig) -> anyhow::Result<Client> {
  if let Some(path) = endpoint_path(&cfg.endpoint) {
    return Client::from_path(path)
      .await
      .with_context(|| format!("connect to containerd path endpoint {}", cfg.endpoint));
  }
  let endpoint = normalize_endpoint(&cfg.endpoint)?;
  let channel = tonic::transport::Endpoint::from_shared(endpoint.clone())
    .with_context(|| format!("parse grpc endpoint {endpoint}"))?
    .connect()
    .await
    .with_context(|| format!("connect containerd grpc endpoint {endpoint}"))?;
  Ok(Client::from(channel))
}

pub(crate) fn resolve_image(req: &DeployWorkloadRequest) -> String {
  req
    .image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .unwrap_or("docker.io/library/nginx:stable-alpine")
    .to_string()
}

pub(crate) fn resolve_backend(req: &DeployWorkloadRequest, service_port: u16) -> String {
  req
    .backend
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .unwrap_or(&format!("127.0.0.1:{service_port}"))
    .to_string()
}

pub(crate) fn container_name(workload_id: &str) -> String {
  let normalized = workload_id
    .trim()
    .chars()
    .map(|ch| {
      if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
        ch.to_ascii_lowercase()
      } else {
        '-'
      }
    })
    .collect::<String>();
  format!("warden-{normalized}")
}

pub(crate) fn looks_like_filesystem_path(value: &str) -> bool {
  let raw = value.trim();
  raw.starts_with('/')
    || raw.starts_with("./")
    || raw.starts_with("../")
    || raw.starts_with(r"\\")
    || (raw.len() > 2 && raw.as_bytes()[1] == b':' && raw.as_bytes()[2] == b'\\')
}

fn ignore_not_found<T>(result: Result<T, tonic::Status>) -> anyhow::Result<Option<T>> {
  match result {
    Ok(value) => Ok(Some(value)),
    Err(err) if err.code() == tonic::Code::NotFound => Ok(None),
    Err(err) => Err(anyhow!(err)),
  }
}

fn endpoint_path(endpoint: &str) -> Option<String> {
  let value = endpoint.trim();
  if let Some(suffix) = value.strip_prefix("unix://") {
    return Some(suffix.to_string());
  }
  if let Some(suffix) = value.strip_prefix("npipe://") {
    return Some(normalize_npipe_path(suffix));
  }
  if let Some(suffix) = value.strip_prefix("npipe:") {
    return Some(normalize_npipe_path(suffix));
  }
  if value.starts_with(r"\\.\pipe\") || value.starts_with('/') {
    return Some(value.to_string());
  }
  None
}

fn normalize_npipe_path(raw: &str) -> String {
  let value = raw.trim().trim_start_matches('/').replace('/', "\\");
  if value.starts_with(r"\\.\pipe\") {
    return value;
  }
  let cleaned = value
    .trim_start_matches(".\\")
    .trim_start_matches("pipe\\")
    .trim_start_matches('\\');
  format!(r"\\.\pipe\{cleaned}")
}

fn normalize_endpoint(value: &str) -> anyhow::Result<String> {
  let endpoint = value.trim().trim_end_matches('/');
  if endpoint.starts_with("http://") || endpoint.starts_with("https://") {
    return Ok(endpoint.to_string());
  }
  Err(anyhow!("unsupported containerd endpoint: {value}"))
}

fn read_env(key: &str) -> Option<String> {
  std::env::var(key)
    .ok()
    .map(|v| v.trim().to_string())
    .filter(|v| !v.is_empty())
}

fn default_endpoint() -> &'static str {
  #[cfg(windows)]
  {
    "npipe:////./pipe/containerd-containerd"
  }
  #[cfg(not(windows))]
  {
    "unix:///run/containerd/containerd.sock"
  }
}
