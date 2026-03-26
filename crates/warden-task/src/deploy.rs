use crate::TaskService;
use chrono::Utc;
use std::collections::{BTreeMap, BTreeSet};
use tracing::{info, warn};
use warden_types::{
  DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
  dsl::DslIngressRouteRecord,
};

#[derive(Debug, Clone, Copy)]
struct TaskDeployOptions {
  publish_ingress: bool,
}

impl Default for TaskDeployOptions {
  fn default() -> Self {
    Self {
      publish_ingress: true,
    }
  }
}

impl TaskService {
  pub async fn deploy(&self, req: DeployWorkloadRequest) -> anyhow::Result<WorkloadSummary> {
    self
      .deploy_with_options(req, TaskDeployOptions::default())
      .await
  }

  pub async fn deploy_from_dsl(
    &self,
    req: DeployWorkloadRequest,
  ) -> anyhow::Result<WorkloadSummary> {
    self
      .deploy_with_options(
        req,
        TaskDeployOptions {
          publish_ingress: false,
        },
      )
      .await
  }

  async fn deploy_with_options(
    &self,
    req: DeployWorkloadRequest,
    options: TaskDeployOptions,
  ) -> anyhow::Result<WorkloadSummary> {
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
            endpoint_name: String::from("http"),
            protocol: String::from("http"),
            address: launched.backend_address.clone(),
            healthy: true,
            ready: true,
            updated_at: Utc::now(),
          })
          .await?;
        if options.publish_ingress {
          self
            .store
            .upsert_route(RouteRecord {
              id: format!("route-{id}"),
              protocol: String::from("http"),
              host: host.clone(),
              path_prefix,
              listen_port: ingress_port,
              backend: launched.backend_address.clone(),
              backend_workload_id: Some(id.clone()),
              backend_endpoint_name: Some(String::from("http")),
              enabled: true,
            })
            .await?;
          if let Some(ip) = extract_backend_ip(&launched.backend_address) {
            self
              .store
              .upsert_dns_record(DnsRecord {
                domain: host,
                values: vec![ip],
                ttl: 60,
              })
              .await?;
          }
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

  pub async fn sync_dsl_ingress_routes(
    &self,
    workload_ids: &[String],
    desired_routes: Vec<DslIngressRouteRecord>,
  ) -> anyhow::Result<()> {
    let managed_workload_ids = workload_ids
      .iter()
      .map(|item| item.trim())
      .filter(|item| !item.is_empty())
      .map(ToString::to_string)
      .collect::<BTreeSet<_>>()
      .into_iter()
      .collect::<Vec<_>>();
    let desired_routes = desired_routes;

    self
      .apply_write("task.sync_dsl_ingress_routes", || async move {
        let mut stale_hosts = BTreeSet::new();
        for workload_id in &managed_workload_ids {
          let existing = self
            .store
            .list_routes_by_backend_workload(workload_id)
            .await;
          for route in existing {
            stale_hosts.insert(route.host.clone());
            self.store.delete_route(&route.id).await?;
          }
        }

        for host in stale_hosts {
          let _ = self.store.delete_dns_record_by_domain(&host).await;
        }

        for item in &desired_routes {
          self.store.upsert_route(item.route.clone()).await?;
        }

        let mut dns_by_host = BTreeMap::<String, (u32, BTreeSet<String>)>::new();
        for item in &desired_routes {
          if !item.dns_enabled {
            continue;
          }
          let Some(workload_id) = item.route.backend_workload_id.as_deref() else {
            continue;
          };
          let entry = dns_by_host
            .entry(item.route.host.clone())
            .or_insert_with(|| (item.dns_ttl, BTreeSet::new()));
          entry.0 = item.dns_ttl;
          for endpoint in self.store.list_endpoints_by_workload(workload_id).await {
            if let Some(ip) = extract_backend_ip(&endpoint.address) {
              entry.1.insert(ip);
            }
          }
        }

        for (domain, (ttl, values)) in dns_by_host {
          if values.is_empty() {
            let _ = self.store.delete_dns_record_by_domain(&domain).await;
            continue;
          }
          self
            .store
            .upsert_dns_record(DnsRecord {
              domain,
              values: values.into_iter().collect(),
              ttl,
            })
            .await?;
        }

        Ok(())
      })
      .await
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

        let routes = self.store.list_routes_by_backend_workload(id).await;
        self.store.delete_endpoints_by_workload(id).await?;
        for route in &routes {
          self.store.delete_route(&route.id).await?;
        }
        for host in routes
          .into_iter()
          .map(|item| item.host)
          .collect::<BTreeSet<_>>()
        {
          let host_still_bound = self
            .store
            .list_routes()
            .await
            .into_iter()
            .any(|route| route.host == host);
          if !host_still_bound {
            let _ = self.store.delete_dns_record_by_domain(&host).await;
          }
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

fn extract_backend_ip(raw: &str) -> Option<String> {
  let backend = raw.trim().trim_start_matches("http://");
  let host = backend.split('/').next().unwrap_or_default();
  let candidate = host.split(':').next().unwrap_or_default().trim();
  if candidate.parse::<std::net::IpAddr>().is_ok() {
    Some(candidate.to_string())
  } else {
    None
  }
}
