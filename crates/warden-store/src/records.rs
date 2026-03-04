use crate::StateStore;
use crate::backend::{PREFIX_DNS, PREFIX_ENDPOINTS, PREFIX_ROUTES, PREFIX_WORKLOADS};
use chrono::Utc;
use tracing::warn;
use warden_types::{DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary};

impl StateStore {
  pub async fn seed_demo_data(&self) -> anyhow::Result<()> {
    if self.has_prefix(PREFIX_WORKLOADS)? {
      return Ok(());
    }

    let now = Utc::now();
    self
      .upsert_workload(WorkloadSummary {
        id: String::from("wk-nginx-01"),
        name: String::from("nginx-demo"),
        runtime: String::from("docker"),
        status: String::from("running"),
        node_id: String::from("node-1"),
        created_at: now,
      })
      .await?;

    self
      .upsert_endpoint(EndpointRecord {
        workload_id: String::from("wk-nginx-01"),
        node_id: String::from("node-1"),
        protocol: String::from("http"),
        address: String::from("10.88.0.10:80"),
      })
      .await?;

    self
      .upsert_route(RouteRecord {
        id: String::from("route-nginx-http"),
        protocol: String::from("http"),
        host: String::from("nginx.local"),
        path_prefix: String::from("/"),
        listen_port: 8088,
        backend: String::from("10.88.0.10:80"),
        enabled: true,
      })
      .await?;

    self
      .upsert_dns_record(DnsRecord {
        domain: String::from("nginx.local"),
        values: vec![String::from("10.88.0.10")],
        ttl: 60,
      })
      .await?;

    Ok(())
  }

  pub async fn upsert_workload(&self, item: WorkloadSummary) -> anyhow::Result<()> {
    self.put_json(&format!("{PREFIX_WORKLOADS}{}", item.id), &item)
  }

  pub async fn upsert_endpoint(&self, item: EndpointRecord) -> anyhow::Result<()> {
    self.put_json(
      &format!(
        "{PREFIX_ENDPOINTS}{}|{}|{}",
        item.workload_id, item.node_id, item.protocol
      ),
      &item,
    )
  }

  pub async fn upsert_route(&self, item: RouteRecord) -> anyhow::Result<()> {
    self.put_json(&format!("{PREFIX_ROUTES}{}", item.id), &item)
  }

  pub async fn upsert_dns_record(&self, item: DnsRecord) -> anyhow::Result<()> {
    self.put_json(&format!("{PREFIX_DNS}{}", item.domain), &item)
  }

  pub async fn delete_workload(&self, id: &str) -> anyhow::Result<()> {
    self.backend.delete(&format!("{PREFIX_WORKLOADS}{id}"))
  }

  pub async fn get_workload(&self, id: &str) -> Option<WorkloadSummary> {
    let key = format!("{PREFIX_WORKLOADS}{id}");
    match self.backend.get(&key) {
      Ok(Some(payload)) => serde_json::from_slice::<WorkloadSummary>(&payload).ok(),
      _ => None,
    }
  }

  pub async fn list_workloads(&self) -> Vec<WorkloadSummary> {
    load_sorted(
      self,
      PREFIX_WORKLOADS,
      |a: &WorkloadSummary, b: &WorkloadSummary| a.name.cmp(&b.name),
      "workloads",
    )
  }

  pub async fn list_endpoints(&self) -> Vec<EndpointRecord> {
    load_sorted(
      self,
      PREFIX_ENDPOINTS,
      |a: &EndpointRecord, b: &EndpointRecord| a.workload_id.cmp(&b.workload_id),
      "endpoints",
    )
  }

  pub async fn list_routes(&self) -> Vec<RouteRecord> {
    load_sorted(
      self,
      PREFIX_ROUTES,
      |a: &RouteRecord, b: &RouteRecord| a.id.cmp(&b.id),
      "routes",
    )
  }

  pub async fn list_dns_records(&self) -> Vec<DnsRecord> {
    load_sorted(
      self,
      PREFIX_DNS,
      |a: &DnsRecord, b: &DnsRecord| a.domain.cmp(&b.domain),
      "dns records",
    )
  }
}

fn load_sorted<T, F>(store: &StateStore, prefix: &str, mut sorter: F, name: &str) -> Vec<T>
where
  T: serde::de::DeserializeOwned,
  F: FnMut(&T, &T) -> std::cmp::Ordering,
{
  match store.load_list::<T>(prefix) {
    Ok(mut data) => {
      data.sort_by(|a, b| sorter(a, b));
      data
    }
    Err(err) => {
      warn!(target: "warden::store", error = %err, "list {name} failed");
      Vec::new()
    }
  }
}
