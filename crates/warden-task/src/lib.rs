mod deploy;
mod logs;
mod scheduling;
mod scheduling_helper;

use std::collections::{HashMap, HashSet};
use std::future::Future;
use tracing::info;
use warden_raft::RaftService;
use warden_runtime::RuntimeEngine;
use warden_store::StateStore;
use warden_types::WorkloadSummary;

#[derive(Debug, Clone)]
pub struct TaskService {
  runtime: RuntimeEngine,
  store: StateStore,
  raft: Option<RaftService>,
  local_node_id: String,
  worker_nodes: Vec<String>,
}

impl TaskService {
  pub fn new(runtime: RuntimeEngine, store: StateStore) -> Self {
    Self::with_nodes(runtime, store, None, String::from("node-1"), Vec::new())
  }

  pub fn with_nodes(
    runtime: RuntimeEngine,
    store: StateStore,
    raft: Option<RaftService>,
    local_node_id: String,
    worker_nodes: Vec<String>,
  ) -> Self {
    let local = normalize_node(&local_node_id).unwrap_or_else(|| String::from("node-1"));
    let mut seen = HashSet::new();
    let mut nodes = worker_nodes
      .into_iter()
      .filter_map(|v| normalize_node(&v))
      .filter(|v| seen.insert(v.clone()))
      .collect::<Vec<_>>();
    if !nodes.iter().any(|node| node == &local) {
      nodes.push(local.clone());
    }
    Self {
      runtime,
      store,
      raft,
      local_node_id: local,
      worker_nodes: nodes,
    }
  }

  pub fn with_raft(
    runtime: RuntimeEngine,
    store: StateStore,
    raft: RaftService,
    local_node_id: String,
    worker_nodes: Vec<String>,
  ) -> Self {
    Self::with_nodes(runtime, store, Some(raft), local_node_id, worker_nodes)
  }

  pub async fn start(&self) {
    info!(
      target: "warden::task",
      local_node_id = %self.local_node_id,
      worker_nodes = ?self.worker_nodes,
      "task service startup begin"
    );
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
    let mut restored = 0usize;
    for item in workloads {
      if !item.status.eq_ignore_ascii_case("running") {
        continue;
      }
      self.runtime.recover_managed(&item.id, &item.runtime).await;
      restored += 1;
    }
    info!(
      target: "warden::task",
      restored,
      "restore managed workloads finished"
    );
  }

  pub(crate) async fn known_nodes(&self) -> Vec<String> {
    let workloads = self.store.list_workloads().await;
    let endpoints = self.store.list_endpoints().await;
    let mut seen = HashSet::new();
    let mut nodes = Vec::new();

    for node in &self.worker_nodes {
      if seen.insert(node.clone()) {
        nodes.push(node.clone());
      }
    }
    if seen.insert(self.local_node_id.clone()) {
      nodes.push(self.local_node_id.clone());
    }
    for item in workloads {
      if seen.insert(item.node_id.clone()) {
        nodes.push(item.node_id);
      }
    }
    for item in endpoints {
      if seen.insert(item.node_id.clone()) {
        nodes.push(item.node_id);
      }
    }

    nodes.sort();
    nodes
  }

  pub(crate) async fn pick_deploy_node(&self) -> String {
    let workloads = self.store.list_workloads().await;
    let nodes = self.known_nodes().await;
    least_loaded_node(&nodes, &workloads).unwrap_or_else(|| self.local_node_id.clone())
  }

  pub(crate) async fn pick_failover_node(&self, failed: &str) -> Option<String> {
    let workloads = self.store.list_workloads().await;
    let nodes = self
      .known_nodes()
      .await
      .into_iter()
      .filter(|node| node != failed)
      .collect::<Vec<_>>();
    least_loaded_node(&nodes, &workloads)
  }

  pub(crate) async fn node_loads(&self) -> HashMap<String, usize> {
    let workloads = self.store.list_workloads().await;
    let nodes = self.known_nodes().await;
    node_load_map(&nodes, &workloads)
  }

  pub(crate) async fn apply_write<T, F, Fut>(&self, op: &str, write: F) -> anyhow::Result<T>
  where
    F: FnOnce() -> Fut,
    Fut: Future<Output = anyhow::Result<T>>,
  {
    match &self.raft {
      Some(raft) => raft.apply_write(op, write).await,
      None => write().await,
    }
  }
}

fn normalize_node(raw: &str) -> Option<String> {
  let value = raw.trim();
  if value.is_empty() {
    None
  } else {
    Some(value.to_string())
  }
}

fn least_loaded_node(nodes: &[String], workloads: &[WorkloadSummary]) -> Option<String> {
  let counts = node_load_map(nodes, workloads);
  counts
    .into_iter()
    .min_by(|a, b| a.1.cmp(&b.1).then_with(|| a.0.cmp(&b.0)))
    .map(|(node, _)| node)
}

fn node_load_map(nodes: &[String], workloads: &[WorkloadSummary]) -> HashMap<String, usize> {
  let mut counts = nodes
    .iter()
    .map(|node| (node.clone(), 0usize))
    .collect::<HashMap<_, _>>();
  for item in workloads
    .iter()
    .filter(|w| w.status.eq_ignore_ascii_case("running"))
  {
    *counts.entry(item.node_id.clone()).or_insert(0) += 1;
  }
  counts
}
