use crate::StateStore;
use crate::backend::{PREFIX_DNS, PREFIX_ENDPOINTS, PREFIX_ROUTES};
use tracing::warn;
use warden_types::{EndpointRecord, RouteRecord};

impl StateStore {
  pub async fn list_endpoints_by_workload(&self, workload_id: &str) -> Vec<EndpointRecord> {
    let prefix = format!("{PREFIX_ENDPOINTS}{workload_id}|");
    match self.load_list::<EndpointRecord>(&prefix) {
      Ok(items) => items,
      Err(err) => {
        warn!(
          target: "warden::store",
          error = %err,
          workload_id = %workload_id,
          "list endpoints by workload failed"
        );
        Vec::new()
      }
    }
  }

  pub async fn delete_endpoints_by_workload(&self, workload_id: &str) -> anyhow::Result<usize> {
    let prefix = format!("{PREFIX_ENDPOINTS}{workload_id}|");
    let rows = self.backend.scan_prefix(&prefix)?;
    let count = rows.len();
    for (key, _) in rows {
      self.backend.delete(&key)?;
    }
    Ok(count)
  }

  pub async fn replace_workload_endpoint_node(
    &self,
    workload_id: &str,
    target_node: &str,
  ) -> anyhow::Result<usize> {
    let endpoints = self.list_endpoints_by_workload(workload_id).await;
    if endpoints.is_empty() {
      return Ok(0);
    }
    let count = endpoints.len();
    self.delete_endpoints_by_workload(workload_id).await?;
    for mut endpoint in endpoints {
      endpoint.node_id = target_node.to_string();
      self.upsert_endpoint(endpoint).await?;
    }
    Ok(count)
  }

  pub async fn get_route_by_workload(&self, workload_id: &str) -> Option<RouteRecord> {
    let key = route_key(workload_id);
    match self.backend.get(&key) {
      Ok(Some(payload)) => serde_json::from_slice::<RouteRecord>(&payload).ok(),
      _ => None,
    }
  }

  pub async fn delete_route_by_workload(&self, workload_id: &str) -> anyhow::Result<()> {
    self.backend.delete(&route_key(workload_id))
  }

  pub async fn delete_dns_record_by_domain(&self, domain: &str) -> anyhow::Result<()> {
    self.backend.delete(&format!("{PREFIX_DNS}{domain}"))
  }
}

fn route_key(workload_id: &str) -> String {
  format!("{PREFIX_ROUTES}route-{workload_id}")
}
