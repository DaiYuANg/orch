mod engine;

use anyhow::anyhow;
use async_trait::async_trait;
use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;
use tokio::sync::RwLock;
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone)]
pub struct RuntimeLaunchResult {
  pub backend_address: String,
}

#[async_trait]
pub trait RuntimeProvider: Send + Sync {
  fn name(&self) -> &'static str;

  async fn start(&self) -> anyhow::Result<()> {
    Ok(())
  }

  async fn deploy(
    &self,
    workload_id: &str,
    req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult>;

  async fn stop(&self, workload_id: &str) -> anyhow::Result<()>;

  async fn logs(&self, workload_id: &str, tail: usize) -> anyhow::Result<Vec<String>> {
    let _ = (workload_id, tail);
    Err(anyhow!("runtime logs not supported"))
  }
}

#[derive(Clone, Default)]
pub struct RuntimeEngine {
  providers: Arc<RwLock<HashMap<String, Arc<dyn RuntimeProvider>>>>,
  managed: Arc<RwLock<HashMap<String, String>>>,
}

impl fmt::Debug for RuntimeEngine {
  fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
    f.debug_struct("RuntimeEngine").finish()
  }
}
