mod helper;

use anyhow::Context;
use async_trait::async_trait;
use tracing::{info, warn};
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
    let image = helper::resolve_image(req);
    let service_port = req.service_port.unwrap_or(80);
    let backend = helper::resolve_backend(req, service_port);
    helper::check_connection(&self.cfg)
      .await
      .context("verify containerd grpc connectivity before deploy")?;

    warn!(
      target: "warden::runtime::containerd",
      workload_id = %workload_id,
      image = %image,
      endpoint = %self.cfg.endpoint,
      "containerd direct grpc deploy is not implemented yet"
    );
    Err(anyhow::anyhow!(
      "containerd direct grpc deploy is not implemented yet: workload_id={} image={} backend={}",
      workload_id,
      image,
      backend
    ))
  }

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    let container_name = helper::container_name(workload_id);
    helper::remove_existing(&self.cfg, &container_name)
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
