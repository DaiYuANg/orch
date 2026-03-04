use anyhow::anyhow;
use async_trait::async_trait;
use tracing::info;
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Default)]
pub struct FirecrackerRuntimeProvider;

impl FirecrackerRuntimeProvider {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl RuntimeProvider for FirecrackerRuntimeProvider {
    fn name(&self) -> &'static str {
        "firecracker"
    }

    async fn start(&self) -> anyhow::Result<()> {
        info!(target: "warden::runtime::firecracker", "firecracker runtime provider startup");
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
            target: "warden::runtime::firecracker",
            workload_id = %workload_id,
            "firecracker stop no-op (not implemented)"
        );
        Ok(())
    }
}
