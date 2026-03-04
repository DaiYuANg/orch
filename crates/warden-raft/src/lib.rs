use anyhow::Context;
use openraft::Config as OpenRaftConfig;
use std::future::Future;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use tracing::{info, warn};

#[derive(Debug, Clone)]
pub struct RaftService {
  pub enabled: bool,
  pub node_id: u64,
  pub bind_addr: String,
  raft_config: Arc<OpenRaftConfig>,
  is_leader: Arc<AtomicBool>,
  applied_index: Arc<AtomicU64>,
}

impl RaftService {
  pub fn new(enabled: bool, node_id: u64, bind_addr: String) -> anyhow::Result<Self> {
    let config = OpenRaftConfig {
      cluster_name: String::from("warden"),
      heartbeat_interval: 500,
      election_timeout_min: 1500,
      election_timeout_max: 3000,
      ..Default::default()
    }
    .validate()
    .context("validate openraft config")?;

    Ok(Self {
      enabled,
      node_id,
      bind_addr,
      raft_config: Arc::new(config),
      is_leader: Arc::new(AtomicBool::new(false)),
      applied_index: Arc::new(AtomicU64::new(0)),
    })
  }

  pub async fn start(&self) {
    if !self.enabled {
      info!(target: "warden::raft", "raft disabled");
      return;
    }
    // Single-node baseline: current node accepts writes as leader.
    self.is_leader.store(true, Ordering::SeqCst);
    info!(
        target: "warden::raft",
        node_id = self.node_id,
        bind_addr = %self.bind_addr,
        is_leader = self.is_leader(),
        cluster = %self.raft_config.cluster_name,
        "openraft write path initialized"
    );
  }

  pub fn is_leader(&self) -> bool {
    !self.enabled || self.is_leader.load(Ordering::SeqCst)
  }

  pub fn applied_index(&self) -> u64 {
    self.applied_index.load(Ordering::SeqCst)
  }

  pub fn leader_node(&self) -> Option<String> {
    if self.is_leader() {
      Some(format!("node-{}", self.node_id))
    } else {
      None
    }
  }

  pub async fn apply_write<T, F, Fut>(&self, op: &str, apply: F) -> anyhow::Result<T>
  where
    F: FnOnce() -> Fut,
    Fut: Future<Output = anyhow::Result<T>>,
  {
    if !self.enabled {
      return apply().await;
    }
    if !self.is_leader() {
      anyhow::bail!("raft write rejected: node-{} is not leader", self.node_id);
    }

    let next_index = self.applied_index.fetch_add(1, Ordering::SeqCst) + 1;
    info!(
      target: "warden::raft",
      node_id = self.node_id,
      log_index = next_index,
      op = op,
      "apply raft write command"
    );

    let result = apply().await;
    if let Err(err) = &result {
      warn!(
        target: "warden::raft",
        node_id = self.node_id,
        log_index = next_index,
        op = op,
        error = %err,
        "raft write command failed"
      );
    }
    result
  }
}
