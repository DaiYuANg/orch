use anyhow::{Context, anyhow};
use async_trait::async_trait;
use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::{info, warn};
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
        self.providers
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
        self.managed
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
        Ok(())
    }

    async fn get_provider(&self, runtime: &str) -> Option<Arc<dyn RuntimeProvider>> {
        self.providers.read().await.get(runtime).cloned()
    }
}
