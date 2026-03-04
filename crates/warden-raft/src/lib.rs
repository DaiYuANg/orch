use anyhow::Context;
use openraft::Config as OpenRaftConfig;
use std::sync::Arc;
use tracing::info;

#[derive(Debug, Clone)]
pub struct RaftService {
    pub enabled: bool,
    pub node_id: u64,
    pub bind_addr: String,
    raft_config: Arc<OpenRaftConfig>,
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
        })
    }

    pub async fn start(&self) {
        if !self.enabled {
            info!(target: "warden::raft", "raft disabled");
            return;
        }
        info!(
            target: "warden::raft",
            node_id = self.node_id,
            bind_addr = %self.bind_addr,
            cluster = %self.raft_config.cluster_name,
            "openraft initialized (foundation mode)"
        );
    }
}
