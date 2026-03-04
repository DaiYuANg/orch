mod deploy;
mod helper;
mod oci;
mod oci_parent;
mod oci_transfer;
mod spec;

use anyhow::Context;
use async_trait::async_trait;
use tracing::info;
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct ContainerdRuntimeProvider {
  cfg: helper::ContainerdRuntimeConfig,
}

impl Default for ContainerdRuntimeProvider {
  fn default() -> Self {
    Self::new()
  }
}

impl ContainerdRuntimeProvider {
  pub fn new() -> Self {
    Self {
      cfg: helper::ContainerdRuntimeConfig::from_env(),
    }
  }
}

#[async_trait]
impl RuntimeProvider for ContainerdRuntimeProvider {
  fn name(&self) -> &'static str {
    "containerd"
  }

  async fn start(&self) -> anyhow::Result<()> {
    helper::check_connection(&self.cfg)
      .await
      .context("check containerd grpc readiness")?;
    info!(
      target: "warden::runtime::containerd",
      endpoint = %self.cfg.endpoint,
      namespace = %self.cfg.namespace,
      "containerd runtime provider startup"
    );
    Ok(())
  }

  async fn deploy(
    &self,
    workload_id: &str,
    req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    let service_port = req.service_port.unwrap_or(80);
    let backend = helper::resolve_backend(req, service_port);
    let image = deploy::deploy_workload(&self.cfg, workload_id, req)
      .await
      .with_context(|| format!("deploy containerd workload {workload_id}"))?;

    info!(
      target: "warden::runtime::containerd",
      workload_id = %workload_id,
      image = %image,
      container = %helper::container_name(workload_id),
      backend = %backend,
      "containerd workload deployed"
    );
    Ok(RuntimeLaunchResult {
      backend_address: backend,
    })
  }

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    let container_name = helper::container_name(workload_id);
    deploy::cleanup_container_and_snapshot(&self.cfg, &container_name)
      .await
      .with_context(|| format!("stop containerd workload {container_name}"))?;
    info!(
      target: "warden::runtime::containerd",
      workload_id = %workload_id,
      container = %container_name,
      "containerd container stopped"
    );
    Ok(())
  }
}
