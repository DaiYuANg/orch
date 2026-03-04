mod deploy;
mod scheduling;

use tracing::info;
use warden_runtime::RuntimeEngine;
use warden_store::StateStore;
use warden_types::WorkloadSummary;

#[derive(Debug, Clone)]
pub struct TaskService {
  runtime: RuntimeEngine,
  store: StateStore,
}

impl TaskService {
  pub fn new(runtime: RuntimeEngine, store: StateStore) -> Self {
    Self { runtime, store }
  }

  pub async fn start(&self) {
    self.runtime.start().await;
    self.restore_managed_workloads().await;
    info!(target: "warden::task", "task service startup complete");
  }

  pub async fn list(&self) -> Vec<WorkloadSummary> {
    self.store.list_workloads().await
  }

  pub async fn get(&self, id: &str) -> Option<WorkloadSummary> {
    self.store.get_workload(id).await
  }

  pub async fn runtime_providers(&self) -> Vec<String> {
    self.runtime.list_providers().await
  }

  pub async fn runtime_managed(&self) -> Vec<(String, String)> {
    self.runtime.managed_summary().await
  }

  async fn restore_managed_workloads(&self) {
    let workloads = self.store.list_workloads().await;
    for item in workloads {
      if !item.status.eq_ignore_ascii_case("running") {
        continue;
      }
      self.runtime.recover_managed(&item.id, &item.runtime).await;
    }
  }
}
