use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiEnvelope<T> {
    pub code: i32,
    pub message: String,
    pub data: T,
}

impl<T> ApiEnvelope<T> {
    pub fn ok(data: T) -> Self {
        Self {
            code: 0,
            message: String::from("ok"),
            data,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkloadSummary {
    pub id: String,
    pub name: String,
    pub runtime: String,
    pub status: String,
    pub node_id: String,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EndpointRecord {
    pub workload_id: String,
    pub node_id: String,
    pub protocol: String,
    pub address: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RouteRecord {
    pub id: String,
    pub protocol: String,
    pub host: String,
    pub path_prefix: String,
    pub listen_port: u16,
    pub backend: String,
    pub enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DnsRecord {
    pub domain: String,
    pub values: Vec<String>,
    pub ttl: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DeployWorkloadRequest {
    pub name: String,
    pub runtime: String,
    pub image: Option<String>,
    pub host: Option<String>,
    pub path_prefix: Option<String>,
    pub service_port: Option<u16>,
    pub ingress_port: Option<u16>,
    pub backend: Option<String>,
}
