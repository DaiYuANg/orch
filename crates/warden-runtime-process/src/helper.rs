use std::collections::BTreeMap;
use std::path::PathBuf;
use std::time::Duration;
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct ProcessRuntimeConfig {
  pub log_dir: PathBuf,
  pub stop_timeout: Duration,
}

impl ProcessRuntimeConfig {
  pub fn from_env() -> Self {
    let stop_timeout_ms = read_env("WARDEN_PROCESS_STOP_TIMEOUT_MS")
      .and_then(|v| v.parse::<u64>().ok())
      .filter(|v| *v > 0)
      .unwrap_or(3_000);
    Self {
      log_dir: read_env("WARDEN_PROCESS_LOG_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| std::env::temp_dir().join("warden-process-logs")),
      stop_timeout: Duration::from_millis(stop_timeout_ms),
    }
  }
}

pub fn resolve_command(req: &DeployWorkloadRequest) -> anyhow::Result<String> {
  req
    .process_command
    .as_deref()
    .map(str::trim)
    .filter(|value| !value.is_empty())
    .map(ToString::to_string)
    .ok_or_else(|| anyhow::anyhow!("process runtime requires process_command"))
}

pub fn resolve_args(req: &DeployWorkloadRequest) -> Vec<String> {
  req
    .process_args
    .iter()
    .map(String::as_str)
    .map(str::trim)
    .filter(|value| !value.is_empty())
    .map(ToString::to_string)
    .collect()
}

pub fn resolve_env(req: &DeployWorkloadRequest) -> BTreeMap<String, String> {
  req
    .process_env
    .iter()
    .filter_map(|(k, v)| {
      let key = k.trim();
      if key.is_empty() {
        return None;
      }
      Some((key.to_string(), v.to_string()))
    })
    .collect()
}

pub fn resolve_cwd(req: &DeployWorkloadRequest) -> Option<PathBuf> {
  req
    .process_cwd
    .as_deref()
    .map(str::trim)
    .filter(|value| !value.is_empty())
    .map(PathBuf::from)
}

pub fn resolve_backend(req: &DeployWorkloadRequest) -> String {
  let service_port = req.service_port.unwrap_or(80);
  req
    .backend
    .as_deref()
    .map(str::trim)
    .filter(|value| !value.is_empty())
    .unwrap_or(&format!("127.0.0.1:{service_port}"))
    .to_string()
}

pub fn sanitize(workload_id: &str) -> String {
  let value = workload_id.trim();
  if value.is_empty() {
    return String::from("workload");
  }
  value
    .chars()
    .map(|ch| {
      if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
        ch.to_ascii_lowercase()
      } else {
        '-'
      }
    })
    .collect()
}

pub fn log_path_for_workload(cfg: &ProcessRuntimeConfig, workload_id: &str) -> PathBuf {
  cfg.log_dir.join(format!("{}.log", sanitize(workload_id)))
}

fn read_env(key: &str) -> Option<String> {
  std::env::var(key)
    .ok()
    .map(|value| value.trim().to_string())
    .filter(|value| !value.is_empty())
}
