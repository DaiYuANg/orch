use std::fs;
use std::path::{Path, PathBuf};
use std::process::Child;
use std::time::{SystemTime, UNIX_EPOCH};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct FirecrackerRuntimeConfig {
  pub binary: String,
  pub log_dir: Option<String>,
}

#[derive(Debug)]
pub struct FirecrackerProcess {
  pub child: Child,
  pub generated_config_path: Option<PathBuf>,
}

impl FirecrackerRuntimeConfig {
  pub fn from_env() -> Self {
    Self {
      binary: read_env("WARDEN_FIRECRACKER_BIN").unwrap_or_else(|| String::from("firecracker")),
      log_dir: read_env("WARDEN_FIRECRACKER_LOG_DIR"),
    }
  }
}

pub fn resolve_config_path(
  req: &DeployWorkloadRequest,
  workload_id: &str,
) -> anyhow::Result<(PathBuf, Option<PathBuf>)> {
  if let Some(path) = req
    .firecracker_config
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
  {
    let target = PathBuf::from(path);
    if !target.exists() {
      anyhow::bail!("firecracker config file not found: {}", target.display());
    }
    return Ok((target, None));
  }

  if let Some(path) = req
    .image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
  {
    let target = PathBuf::from(path);
    if target.exists() {
      return Ok((target, None));
    }
  }

  let kernel = req
    .firecracker_kernel_image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .ok_or_else(missing_firecracker_config)?;
  let rootfs = req
    .firecracker_rootfs
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .ok_or_else(missing_firecracker_config)?;
  if !Path::new(kernel).exists() {
    anyhow::bail!("firecracker kernel image not found: {kernel}");
  }
  if !Path::new(rootfs).exists() {
    anyhow::bail!("firecracker rootfs not found: {rootfs}");
  }

  let dir = std::env::temp_dir().join("warden-firecracker");
  fs::create_dir_all(&dir)?;
  let ts = SystemTime::now()
    .duration_since(UNIX_EPOCH)
    .map(|d| d.as_millis())
    .unwrap_or(0);
  let file = dir.join(format!("{}-{}.json", sanitize(workload_id), ts));
  let config = serde_json::json!({
    "boot-source": {
      "kernel_image_path": kernel,
      "boot_args": "console=ttyS0 reboot=k panic=1 pci=off"
    },
    "drives": [
      {
        "drive_id": "rootfs",
        "path_on_host": rootfs,
        "is_root_device": true,
        "is_read_only": false
      }
    ],
    "machine-config": {
      "vcpu_count": 1,
      "mem_size_mib": 512
    }
  });
  fs::write(&file, serde_json::to_vec_pretty(&config)?)?;
  Ok((file.clone(), Some(file)))
}

pub fn sanitize(workload_id: &str) -> String {
  workload_id
    .trim()
    .chars()
    .map(|ch| {
      if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
        ch.to_ascii_lowercase()
      } else {
        '-'
      }
    })
    .collect::<String>()
}

fn missing_firecracker_config() -> anyhow::Error {
  anyhow::anyhow!(
    "firecracker requires --firecracker-config, or both --firecracker-kernel-image and --firecracker-rootfs"
  )
}

fn read_env(key: &str) -> Option<String> {
  std::env::var(key)
    .ok()
    .map(|v| v.trim().to_string())
    .filter(|v| !v.is_empty())
}
