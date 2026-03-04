use anyhow::anyhow;
use async_trait::async_trait;
use tracing::info;
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Default)]
pub struct ContainerdRuntimeProvider;

impl ContainerdRuntimeProvider {
  pub fn new() -> Self {
    Self
  }
}

#[async_trait]
impl RuntimeProvider for ContainerdRuntimeProvider {
  fn name(&self) -> &'static str {
    "containerd"
  }

  async fn start(&self) -> anyhow::Result<()> {
    info!(target: "warden::runtime::containerd", "containerd runtime provider startup");
    Ok(())
  }

  async fn deploy(
    &self,
    _workload_id: &str,
    _req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    Err(anyhow!(
      "runtime provider '{}' is not implemented yet",
      self.name()
    ))
  }

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    info!(
        target: "warden::runtime::containerd",
        workload_id = %workload_id,
        "containerd stop no-op (not implemented)"
    );
    Ok(())
  }
}
