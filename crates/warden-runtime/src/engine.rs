use crate::{RuntimeEngine, RuntimeLaunchResult, RuntimeProvider};
use anyhow::{Context, anyhow};
use std::sync::Arc;
use tracing::{info, warn};
use warden_types::DeployWorkloadRequest;

impl RuntimeEngine {
  pub fn new() -> Self {
    Self::default()
  }

  pub async fn register_provider(&self, provider: Arc<dyn RuntimeProvider>) {
    let runtime = provider.name().trim().to_ascii_lowercase();
    if runtime.is_empty() {
      warn!(target: "warden::runtime", "skip empty runtime provider name");
      return;
    }
    self
      .providers
      .write()
      .await
      .insert(runtime.clone(), provider);
    info!(
      target: "warden::runtime",
      runtime = %runtime,
      "runtime provider registered"
    );
  }

  pub async fn start(&self) {
    let providers = {
      let map = self.providers.read().await;
      map.values().cloned().collect::<Vec<_>>()
    };
    let names = providers.iter().map(|p| p.name()).collect::<Vec<_>>();
    info!(
      target: "warden::runtime",
      providers = ?names,
      "runtime engine startup"
    );
    for provider in providers {
      if let Err(err) = provider.start().await {
        warn!(
          target: "warden::runtime",
          runtime = provider.name(),
          error = %err,
          "runtime provider startup failed"
        );
      }
    }
  }

  pub async fn deploy(
    &self,
    workload_id: &str,
    req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    let runtime = req.runtime.trim().to_ascii_lowercase();
    if runtime.is_empty() {
      return Err(anyhow!("runtime must not be empty"));
    }
    let provider = self
      .get_provider(&runtime)
      .await
      .with_context(|| format!("runtime provider not found: {runtime}"))?;
    let launched = provider
      .deploy(workload_id, req)
      .await
      .with_context(|| format!("runtime deploy failed for provider {runtime}"))?;
    info!(
      target: "warden::runtime",
      workload_id = %workload_id,
      runtime = %runtime,
      backend = %launched.backend_address,
      "runtime deploy completed"
    );
    self
      .managed
      .write()
      .await
      .insert(workload_id.to_string(), runtime);
    Ok(launched)
  }

  pub async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
    let runtime = {
      let map = self.managed.read().await;
      map.get(workload_id).cloned()
    };

    let Some(runtime) = runtime else {
      info!(
        target: "warden::runtime",
        workload_id = %workload_id,
        "runtime stop no-op for unmanaged resource"
      );
      return Ok(());
    };

    let provider = self.get_provider(&runtime).await;
    let Some(provider) = provider else {
      warn!(
        target: "warden::runtime",
        workload_id = %workload_id,
        runtime = %runtime,
        "runtime provider missing for managed workload"
      );
      self.managed.write().await.remove(workload_id);
      return Ok(());
    };

    provider
      .stop(workload_id)
      .await
      .with_context(|| format!("runtime stop failed for provider {runtime}"))?;
    self.managed.write().await.remove(workload_id);
    info!(
      target: "warden::runtime",
      workload_id = %workload_id,
      runtime = %runtime,
      "runtime stop completed"
    );
    Ok(())
  }

  pub async fn logs(
    &self,
    workload_id: &str,
    runtime: &str,
    tail: usize,
  ) -> anyhow::Result<Vec<String>> {
    let runtime = runtime.trim().to_ascii_lowercase();
    if runtime.is_empty() {
      return Err(anyhow!("runtime must not be empty for logs"));
    }
    let provider = self
      .get_provider(&runtime)
      .await
      .with_context(|| format!("runtime provider not found for logs: {runtime}"))?;
    let lines = provider
      .logs(workload_id, tail)
      .await
      .with_context(|| format!("runtime logs failed for provider {runtime}"))?;
    info!(
      target: "warden::runtime",
      workload_id = %workload_id,
      runtime = %runtime,
      tail,
      lines = lines.len(),
      "runtime logs fetched"
    );
    Ok(lines)
  }

  pub async fn recover_managed(&self, workload_id: &str, runtime: &str) {
    let normalized = runtime.trim().to_ascii_lowercase();
    if normalized.is_empty() {
      return;
    }
    let exists = self.providers.read().await.contains_key(&normalized);
    if !exists {
      warn!(
        target: "warden::runtime",
        workload_id = %workload_id,
        runtime = %normalized,
        "skip managed recovery due to missing provider"
      );
      return;
    }
    self
      .managed
      .write()
      .await
      .insert(workload_id.to_string(), normalized);
  }

  pub async fn list_providers(&self) -> Vec<String> {
    let map = self.providers.read().await;
    let mut names = map.keys().cloned().collect::<Vec<_>>();
    names.sort();
    names
  }

  pub async fn managed_summary(&self) -> Vec<(String, String)> {
    let map = self.managed.read().await;
    let mut rows = map
      .iter()
      .map(|(k, v)| (k.clone(), v.clone()))
      .collect::<Vec<_>>();
    rows.sort_by(|a, b| a.0.cmp(&b.0));
    rows
  }

  async fn get_provider(&self, runtime: &str) -> Option<Arc<dyn RuntimeProvider>> {
    self.providers.read().await.get(runtime).cloned()
  }
}
