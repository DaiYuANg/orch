mod loader;

use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize,Serialize)]
pub struct WardenConfig {
    pub node: NodeConfig,
    pub runtime: RuntimeConfig,
    pub network: NetworkConfig,
    pub store: StoreConfig,
    pub dns: Option<DnsConfig>,
    pub secrets: Option<SecretsConfig>,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct NodeConfig {
    pub name: String,
    pub bind_addr: String,
    pub http_port: u16,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct RuntimeConfig {
    pub temp_dir: String,
    pub log_dir: String,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct NetworkConfig {
    pub advertise_ip: String,
    pub peers: Vec<String>,
    pub gossip_port: u16,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct StoreConfig {
    pub enabled: bool,
    pub backend: String,
    pub path: String,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct DnsConfig {
    pub enabled: bool,
    pub r#type: String,
    pub namespace: Option<String>,
    pub kubeconfig: Option<String>,
}

#[derive(Debug, Deserialize,Serialize)]
pub struct SecretsConfig {
    pub enabled: bool,
    pub mount_dir: String,
    pub inject_env: bool,
}

impl Default for WardenConfig {
    fn default() -> Self {
        WardenConfig{
            node: NodeConfig {
                name: "".to_string(),
                bind_addr: "".to_string(),
                http_port: 0,
            },
            runtime: RuntimeConfig { temp_dir: "".to_string(), log_dir: "".to_string() },
            network: NetworkConfig {
                advertise_ip: "".to_string(),
                peers: vec![],
                gossip_port: 0,
            },
            store: StoreConfig {
                enabled: false,
                backend: "".to_string(),
                path: "".to_string(),
            },
            dns: None,
            secrets: None,
        }
    }
}