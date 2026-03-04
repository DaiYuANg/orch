mod helper;

use async_trait::async_trait;
use std::collections::HashMap;
use std::fs::{self, OpenOptions};
use std::process::{Command, Stdio};
use std::sync::{Arc, Mutex};
use tracing::{info, warn};
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct FirecrackerRuntimeProvider {
  cfg: helper::FirecrackerRuntimeConfig,
  processes: Arc<Mutex<HashMap<String, helper::FirecrackerProcess>>>,
}

impl FirecrackerRuntimeProvider {
  pub fn new() -> Self {
    Self {
      cfg: helper::FirecrackerRuntimeConfig::from_env(),
      processes: Arc::new(Mutex::new(HashMap::new())),
    }
  }
}

impl Default for FirecrackerRuntimeProvider {
  fn default() -> Self {
    Self::new()
  }
}

#[async_trait]
impl RuntimeProvider for FirecrackerRuntimeProvider {
  fn name(&self) -> &'static str {
    "firecracker"
  }

  async fn start(&self) -> anyhow::Result<()> {
    info!(
      target: "warden::runtime::firecracker",
      binary = %self.cfg.binary,
      log_dir = ?self.cfg.log_dir,
      "firecracker runtime provider startup"
    );
    Ok(())
  }

  async fn deploy(
    &self,
    workload_id: &str,
    req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    let (config_path, generated_config_path) = helper::resolve_config_path(req, workload_id)?;

    let mut cmd = Command::new(&self.cfg.binary);
    cmd
      .arg("--config-file")
      .arg(&config_path)
      .stdin(Stdio::null())
      .stdout(Stdio::null())
      .stderr(Stdio::null());

    if let Some(dir) = self.cfg.log_dir.as_deref() {
      fs::create_dir_all(dir)?;
      let file_name = format!("{}.log", helper::sanitize(workload_id));
      let path = std::path::Path::new(dir).join(file_name);
      let file = OpenOptions::new().create(true).append(true).open(path)?;
      let err_file = file.try_clone()?;
      cmd.stdout(Stdio::from(file)).stderr(Stdio::from(err_file));
    }

    let child = cmd
      .spawn()
      .map_err(|err| anyhow::anyhow!("spawn firecracker failed: {err}"))?;

    let mut guard = self
      .processes
      .lock()
      .map_err(|_| anyhow::anyhow!("firecracker process lock poisoned"))?;
    if let Some(mut old) = guard.insert(
      workload_id.to_string(),
      helper::FirecrackerProcess {
        child,
        generated_config_path,
      },
    ) {
      let _ = old.child.kill();
      let _ = old.child.wait();
      if let Some(path) = old.generated_config_path.take() {
        let _ = fs::remove_file(path);
      }
    }

    let service_port = req.service_port.unwrap_or(80);
    let backend = req
      .backend
      .as_deref()
      .map(str::trim)
      .filter(|v| !v.is_empty())
      .unwrap_or(&format!("127.0.0.1:{service_port}"))
      .to_string();

    info!(
      target: "warden::runtime::firecracker",
      workload_id = %workload_id,
      config = %config_path.display(),
      backend = %backend,
      "firecracker workload launched"
    );

    Ok(RuntimeLaunchResult {
      backend_address: backend,
    })
  }

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    let process = {
      let mut guard = self
        .processes
        .lock()
        .map_err(|_| anyhow::anyhow!("firecracker process lock poisoned"))?;
      guard.remove(workload_id)
    };
    let Some(mut process) = process else {
      warn!(
        target: "warden::runtime::firecracker",
        workload_id = %workload_id,
        "firecracker stop no-op for unmanaged workload"
      );
      return Ok(());
    };

    let _ = process.child.kill();
    let _ = process.child.wait();
    if let Some(path) = process.generated_config_path.take() {
      let _ = fs::remove_file(path);
    }
    info!(
      target: "warden::runtime::firecracker",
      workload_id = %workload_id,
      "firecracker workload stopped"
    );
    Ok(())
  }
}
