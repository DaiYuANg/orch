use crate::TaskService;
use chrono::Utc;
use tracing::{info, warn};
use warden_types::{
  DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
};

impl TaskService {
  pub async fn deploy(&self, req: DeployWorkloadRequest) -> anyhow::Result<WorkloadSummary> {
    let id = generate_workload_id(&req.name);
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
    let target_node = self.pick_deploy_node().await;
    info!(
      target: "warden::task",
      workload_id = %id,
      name = %req.name,
      runtime = %runtime,
      target_node = %target_node,
      "deploy request accepted"
    );
    let launched = self.runtime.deploy(&id, &req).await?;

    let summary = self
      .apply_write("task.deploy", || async {
        let summary = WorkloadSummary {
          id: id.clone(),
          name: req.name.trim().to_string(),
          runtime: runtime.clone(),
          status: String::from("running"),
          node_id: target_node.clone(),
          created_at: Utc::now(),
        };
        self.store.upsert_workload(summary.clone()).await?;
        self
          .store
          .upsert_endpoint(EndpointRecord {
            workload_id: id.clone(),
            node_id: target_node,
            protocol: String::from("http"),
            address: launched.backend_address.clone(),
          })
          .await?;
        self
          .store
          .upsert_route(RouteRecord {
            id: format!("route-{id}"),
            protocol: String::from("http"),
            host: host.clone(),
            path_prefix,
            listen_port: ingress_port,
            backend: launched.backend_address.clone(),
            enabled: true,
          })
          .await?;

        if let Some(ip) = extract_host_ip(&launched.backend_address) {
          self
            .store
            .upsert_dns_record(DnsRecord {
              domain: host,
              values: vec![ip],
              ttl: 60,
            })
            .await?;
        }
        Ok(summary)
      })
      .await?;

    info!(
      target: "warden::task",
      workload_id = %summary.id,
      runtime = %runtime,
      backend = %launched.backend_address,
      "workload deployed"
    );
    Ok(summary)
  }

  pub async fn stop(&self, id: &str) -> anyhow::Result<Option<WorkloadSummary>> {
    let mut workload = match self.store.get_workload(id).await {
      Some(item) => item,
      None => {
        warn!(
          target: "warden::task",
          workload_id = %id,
          "stop requested for missing workload"
        );
        return Ok(None);
      }
    };
    self.runtime.stop(id).await?;
    workload.status = String::from("stopped");
    let stopped = self
      .apply_write("task.stop", || async {
        self.store.upsert_workload(workload.clone()).await?;

        let route = self.store.get_route_by_workload(id).await;
        self.store.delete_endpoints_by_workload(id).await?;
        self.store.delete_route_by_workload(id).await?;
        if let Some(item) = route {
          let _ = self.store.delete_dns_record_by_domain(&item.host).await;
        }
        Ok(workload)
      })
      .await?;

    info!(target: "warden::task", workload_id = %id, "workload stopped");
    Ok(Some(stopped))
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
