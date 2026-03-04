use chrono::Utc;
use tracing::info;
use warden_runtime::RuntimeEngine;
use warden_store::StateStore;
use warden_types::{
    DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
};

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
        info!(target: "warden::task", "task service startup complete");
    }

    pub async fn list(&self) -> Vec<WorkloadSummary> {
        self.store.list_workloads().await
    }

    pub async fn get(&self, id: &str) -> Option<WorkloadSummary> {
        self.store.get_workload(id).await
    }

    pub async fn deploy(&self, req: DeployWorkloadRequest) -> anyhow::Result<WorkloadSummary> {
        let id = generate_workload_id(&req.name);
        let created_at = Utc::now();
        let runtime = normalize_or_default(req.runtime.trim(), "docker");
        let host = req
            .host
            .as_deref()
            .map(str::trim)
            .filter(|v| !v.is_empty())
            .map(ToString::to_string)
            .unwrap_or_else(|| format!("{id}.warden.local"));
        let path_prefix = req
            .path_prefix
            .as_deref()
            .map(str::trim)
            .filter(|v| !v.is_empty())
            .map(ToString::to_string)
            .unwrap_or_else(|| String::from("/"));
        let ingress_port = req.ingress_port.unwrap_or(8088);

        let summary = WorkloadSummary {
            id: id.clone(),
            name: req.name.trim().to_string(),
            runtime: runtime.clone(),
            status: String::from("running"),
            node_id: String::from("node-1"),
            created_at,
        };

        let launched = self.runtime.deploy(&id, &req).await?;
        let backend = launched.backend_address;

        self.store.upsert_workload(summary.clone()).await?;
        self.store
            .upsert_endpoint(EndpointRecord {
                workload_id: id.clone(),
                node_id: String::from("node-1"),
                protocol: String::from("http"),
                address: backend.clone(),
            })
            .await?;
        self.store
            .upsert_route(RouteRecord {
                id: format!("route-{id}"),
                protocol: String::from("http"),
                host: host.clone(),
                path_prefix,
                listen_port: ingress_port,
                backend: backend.clone(),
                enabled: true,
            })
            .await?;

        if let Some(ip) = extract_host_ip(&backend) {
            self.store
                .upsert_dns_record(DnsRecord {
                    domain: host,
                    values: vec![ip],
                    ttl: 60,
                })
                .await?;
        }

        info!(
            target: "warden::task",
            workload_id = %summary.id,
            runtime = %runtime,
            backend = %backend,
            "workload deployed"
        );
        Ok(summary)
    }

    pub async fn stop(&self, id: &str) -> anyhow::Result<Option<WorkloadSummary>> {
        let mut workload = match self.store.get_workload(id).await {
            Some(item) => item,
            None => return Ok(None),
        };
        self.runtime.stop(id).await?;
        workload.status = String::from("stopped");
        self.store.upsert_workload(workload.clone()).await?;
        info!(target: "warden::task", workload_id = %id, "workload stopped");
        Ok(Some(workload))
    }
}

fn generate_workload_id(name: &str) -> String {
    let base = name
        .chars()
        .map(|ch| {
            if ch.is_ascii_alphanumeric() {
                ch.to_ascii_lowercase()
            } else {
                '-'
            }
        })
        .collect::<String>()
        .trim_matches('-')
        .to_string();
    let suffix = Utc::now().timestamp_millis();
    if base.is_empty() {
        format!("wk-{suffix}")
    } else {
        format!("wk-{base}-{suffix}")
    }
}

fn normalize_or_default(value: &str, fallback: &str) -> String {
    if value.is_empty() {
        fallback.to_string()
    } else {
        value.to_string()
    }
}

fn extract_host_ip(raw: &str) -> Option<String> {
    let backend = raw.trim().trim_start_matches("http://");
    let host = backend.split('/').next().unwrap_or_default();
    let candidate = host.split(':').next().unwrap_or_default().trim();
    if candidate.parse::<std::net::IpAddr>().is_ok() {
        Some(candidate.to_string())
    } else {
        None
    }
}
