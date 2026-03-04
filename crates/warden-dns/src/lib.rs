use anyhow::Context;
use hickory_proto::rr::Name;
use hickory_server::authority::Catalog;
use hickory_server::server::ServerFuture;
use moka::sync::Cache;
use std::net::SocketAddr;
use std::str::FromStr;
use std::time::Duration;
use tokio::net::{TcpListener, UdpSocket};
use tracing::{error, info, warn};
use warden_store::StateStore;
use warden_types::DnsRecord;

#[derive(Debug, Clone)]
pub struct DnsOptions {
  pub record_cache_capacity: u64,
  pub record_cache_ttl: Duration,
}

impl Default for DnsOptions {
  fn default() -> Self {
    Self {
      record_cache_capacity: 256,
      record_cache_ttl: Duration::from_secs(2),
    }
  }
}

#[derive(Debug, Clone)]
pub struct DnsService {
  store: StateStore,
  record_cache: Cache<String, Vec<DnsRecord>>,
}

impl DnsService {
  pub fn new(store: StateStore) -> Self {
    Self::with_options(store, DnsOptions::default())
  }

  pub fn with_options(store: StateStore, options: DnsOptions) -> Self {
    let cache = Cache::builder()
      .max_capacity(options.record_cache_capacity)
      .time_to_live(options.record_cache_ttl)
      .build();
    Self {
      store,
      record_cache: cache,
    }
  }

  pub async fn start(&self, listen_addr: &str) -> anyhow::Result<()> {
    let addr = normalize_listen_addr(listen_addr)?;
    let records = self.load_records().await;
    if records.is_empty() {
      warn!(target: "warden::dns", %addr, "dns server starts with empty records");
    }

    let catalog = build_catalog(&records)?;
    let mut server = ServerFuture::new(catalog);
    let udp = UdpSocket::bind(addr)
      .await
      .with_context(|| format!("bind dns udp {addr}"))?;
    let tcp = TcpListener::bind(addr)
      .await
      .with_context(|| format!("bind dns tcp {addr}"))?;

    server.register_socket(udp);
    server.register_listener(tcp, Duration::from_secs(10));

    tokio::spawn(async move {
      if let Err(err) = server.block_until_done().await {
        error!(target: "warden::dns", error = %err, "dns server stopped with error");
      }
    });

    info!(
        target: "warden::dns",
        %addr,
        records = records.len(),
        "dns server started with hickory"
    );
    Ok(())
  }

  pub async fn list_records(&self) -> Vec<DnsRecord> {
    self.load_records().await
  }

  async fn load_records(&self) -> Vec<DnsRecord> {
    let key = String::from("all");
    if let Some(records) = self.record_cache.get(&key) {
      return records;
    }
    let records = self.store.list_dns_records().await;
    self.record_cache.insert(key, records.clone());
    records
  }
}

fn build_catalog(records: &[DnsRecord]) -> anyhow::Result<Catalog> {
  let mut catalog = Catalog::new();
  for item in records {
    let zone = ensure_fqdn(&item.domain);
    let name = Name::from_str(&zone).with_context(|| format!("parse dns zone: {}", item.domain))?;
    catalog.upsert(name.into(), vec![]);
  }
  Ok(catalog)
}

fn normalize_listen_addr(raw: &str) -> anyhow::Result<SocketAddr> {
  let trimmed = raw.trim();
  if trimmed.is_empty() {
    return "0.0.0.0:1053"
      .parse()
      .context("parse default dns listen addr");
  }
  if let Some(port) = trimmed.strip_prefix(':') {
    return format!("0.0.0.0:{port}")
      .parse()
      .with_context(|| format!("parse dns listen addr: {trimmed}"));
  }
  trimmed
    .parse()
    .with_context(|| format!("parse dns listen addr: {trimmed}"))
}

fn ensure_fqdn(domain: &str) -> String {
  let value = domain.trim();
  if value.ends_with('.') {
    value.to_string()
  } else {
    format!("{value}.")
  }
}
