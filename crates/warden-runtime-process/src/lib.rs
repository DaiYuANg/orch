mod helper;
mod process_ctl;
mod tail;

use anyhow::Context;
use async_trait::async_trait;
use std::collections::HashMap;
use std::fs::{self, OpenOptions};
use std::path::PathBuf;
use std::process::Stdio;
use std::sync::Arc;
use tokio::process::Command;
use tokio::sync::Mutex;
use tracing::{info, warn};
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

pub struct ProcessRuntimeProvider {
  cfg: helper::ProcessRuntimeConfig,
  processes: Arc<Mutex<HashMap<String, ManagedProcess>>>,
}

struct ManagedProcess {
  child: tokio::process::Child,
  log_path: PathBuf,
}

impl ProcessRuntimeProvider {
  pub fn new() -> Self {
    Self {
      cfg: helper::ProcessRuntimeConfig::from_env(),
      processes: Arc::new(Mutex::new(HashMap::new())),
    }
  }
}

impl Default for ProcessRuntimeProvider {
  fn default() -> Self {
    Self::new()
  }
}

#[async_trait]
impl RuntimeProvider for ProcessRuntimeProvider {
  fn name(&self) -> &'static str {
    "process"
  }

  async fn start(&self) -> anyhow::Result<()> {
    info!(
      target: "warden::runtime::process",
      log_dir = ?self.cfg.log_dir,
      stop_timeout_ms = self.cfg.stop_timeout.as_millis(),
      "process runtime provider startup"
    );
    Ok(())
  }

  async fn deploy(
    &self,
    workload_id: &str,
    req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    let command = helper::resolve_command(req)?;
    let args = helper::resolve_args(req);
    let envs = helper::resolve_env(req);
    let cwd = helper::resolve_cwd(req);
    let backend = helper::resolve_backend(req);

    let mut cmd = Command::new(&command);
    cmd.stdin(Stdio::null()).args(&args);
    if let Some(cwd) = cwd {
      cmd.current_dir(cwd);
    }
    for (key, value) in envs {
      cmd.env(key, value);
    }
    let log_path = configure_stdio(&mut cmd, &self.cfg, workload_id)?;
    let child = cmd
      .spawn()
      .with_context(|| format!("spawn process runtime command: {command}"))?;
    let pid = child.id();

    let previous = {
      let mut guard = self.processes.lock().await;
      guard.insert(
        workload_id.to_string(),
        ManagedProcess {
          child,
          log_path: log_path.clone(),
        },
      )
    };
    if let Some(previous) = previous
      && let Err(err) =
        process_ctl::stop_child(workload_id, previous.child, self.cfg.stop_timeout).await
    {
      warn!(
        target: "warden::runtime::process",
        workload_id = %workload_id,
        error = %err,
        "failed to stop previous process runtime instance"
      );
    }

    info!(
      target: "warden::runtime::process",
      workload_id = %workload_id,
      command = %command,
      args = ?args,
      backend = %backend,
      pid = ?pid,
      log_path = ?log_path,
      "process workload deployed"
    );
    Ok(RuntimeLaunchResult {
      backend_address: backend,
    })
  }

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    let managed = {
      let mut guard = self.processes.lock().await;
      guard.remove(workload_id)
    };
    let Some(managed) = managed else {
      warn!(
        target: "warden::runtime::process",
        workload_id = %workload_id,
        "process stop no-op for unmanaged workload"
      );
      return Ok(());
    };

    let log_path = managed.log_path.clone();
    process_ctl::stop_child(workload_id, managed.child, self.cfg.stop_timeout).await?;
    info!(
      target: "warden::runtime::process",
      workload_id = %workload_id,
      log_path = ?log_path,
      "process workload stopped"
    );
    Ok(())
  }

  async fn logs(&self, workload_id: &str, tail: usize) -> anyhow::Result<Vec<String>> {
    let limit = tail.max(1);
    let path = helper::log_path_for_workload(&self.cfg, workload_id);
    let lines = tail::read_tail_lines(&path, limit)?;
    info!(
      target: "warden::runtime::process",
      workload_id = %workload_id,
      tail = limit,
      lines = lines.len(),
      log_path = %path.display(),
      "process runtime logs fetched"
    );
    Ok(lines)
  }
}

fn configure_stdio(
  cmd: &mut Command,
  cfg: &helper::ProcessRuntimeConfig,
  workload_id: &str,
) -> anyhow::Result<PathBuf> {
  fs::create_dir_all(&cfg.log_dir)?;
  let path = helper::log_path_for_workload(cfg, workload_id);
  let file = OpenOptions::new().create(true).append(true).open(&path)?;
  let err_file = file.try_clone()?;
  cmd.stdout(Stdio::from(file)).stderr(Stdio::from(err_file));
  Ok(path)
}
