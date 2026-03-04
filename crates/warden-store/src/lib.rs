use anyhow::{Context, bail};
use chrono::Utc;
use redb::{Database, ReadableDatabase, ReadableTable, TableDefinition};
use serde::{Serialize, de::DeserializeOwned};
use std::collections::HashMap;
use std::fmt;
use std::path::Path;
use std::sync::{Arc, RwLock};
use tracing::warn;
use warden_types::{DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary};

const PREFIX_WORKLOADS: &str = "workloads/";
const PREFIX_ENDPOINTS: &str = "endpoints/";
const PREFIX_ROUTES: &str = "routes/";
const PREFIX_DNS: &str = "dns/";
const KV_TABLE: TableDefinition<&str, &[u8]> = TableDefinition::new("warden_kv");

pub trait KvBackend: Send + Sync {
    fn get(&self, key: &str) -> anyhow::Result<Option<Vec<u8>>>;
    fn put(&self, key: &str, value: &[u8]) -> anyhow::Result<()>;
    fn delete(&self, key: &str) -> anyhow::Result<()>;
    fn scan_prefix(&self, prefix: &str) -> anyhow::Result<Vec<(String, Vec<u8>)>>;
}

#[derive(Default)]
pub struct MemoryKvBackend {
    inner: RwLock<HashMap<String, Vec<u8>>>,
}

impl MemoryKvBackend {
    pub fn new() -> Self {
        Self::default()
    }
}

impl KvBackend for MemoryKvBackend {
    fn get(&self, key: &str) -> anyhow::Result<Option<Vec<u8>>> {
        let guard = self
            .inner
            .read()
            .map_err(|_| anyhow::anyhow!("memory kv lock poisoned"))?;
        Ok(guard.get(key).cloned())
    }

    fn put(&self, key: &str, value: &[u8]) -> anyhow::Result<()> {
        let mut guard = self
            .inner
            .write()
            .map_err(|_| anyhow::anyhow!("memory kv lock poisoned"))?;
        guard.insert(key.to_string(), value.to_vec());
        Ok(())
    }

    fn delete(&self, key: &str) -> anyhow::Result<()> {
        let mut guard = self
            .inner
            .write()
            .map_err(|_| anyhow::anyhow!("memory kv lock poisoned"))?;
        let _ = guard.remove(key);
        Ok(())
    }

    fn scan_prefix(&self, prefix: &str) -> anyhow::Result<Vec<(String, Vec<u8>)>> {
        let guard = self
            .inner
            .read()
            .map_err(|_| anyhow::anyhow!("memory kv lock poisoned"))?;
        let mut rows: Vec<(String, Vec<u8>)> = guard
            .iter()
            .filter(|(key, _)| key.starts_with(prefix))
            .map(|(key, value)| (key.clone(), value.clone()))
            .collect();
        rows.sort_by(|a, b| a.0.cmp(&b.0));
        Ok(rows)
    }
}

pub struct RedbKvBackend {
    db: Database,
}

impl RedbKvBackend {
    pub fn open(path: impl AsRef<Path>) -> anyhow::Result<Self> {
        let db = Database::create(path.as_ref())
            .with_context(|| format!("open redb file {}", path.as_ref().display()))?;
        let write_txn = db.begin_write().context("start redb init write txn")?;
        {
            let _ = write_txn
                .open_table(KV_TABLE)
                .context("open redb kv table")?;
        }
        write_txn.commit().context("commit redb init txn")?;
        Ok(Self { db })
    }
}

impl KvBackend for RedbKvBackend {
    fn get(&self, key: &str) -> anyhow::Result<Option<Vec<u8>>> {
        let txn = self.db.begin_read().context("redb begin read txn")?;
        let table = txn
            .open_table(KV_TABLE)
            .context("redb open table for read")?;
        let maybe_value = table
            .get(key)
            .with_context(|| format!("redb get key {key}"))?;
        if let Some(value) = maybe_value {
            return Ok(Some(value.value().to_vec()));
        }
        Ok(None)
    }

    fn put(&self, key: &str, value: &[u8]) -> anyhow::Result<()> {
        let txn = self.db.begin_write().context("redb begin write txn")?;
        {
            let mut table = txn
                .open_table(KV_TABLE)
                .context("redb open table for write")?;
            table
                .insert(key, value)
                .with_context(|| format!("redb put key {key}"))?;
        }
        txn.commit().context("redb commit write txn")?;
        Ok(())
    }

    fn delete(&self, key: &str) -> anyhow::Result<()> {
        let txn = self.db.begin_write().context("redb begin delete txn")?;
        {
            let mut table = txn
                .open_table(KV_TABLE)
                .context("redb open table for delete")?;
            let _ = table
                .remove(key)
                .with_context(|| format!("redb delete key {key}"))?;
        }
        txn.commit().context("redb commit delete txn")?;
        Ok(())
    }

    fn scan_prefix(&self, prefix: &str) -> anyhow::Result<Vec<(String, Vec<u8>)>> {
        let txn = self.db.begin_read().context("redb begin scan txn")?;
        let table = txn
            .open_table(KV_TABLE)
            .context("redb open table for scan")?;

        let mut rows = Vec::new();
        for row in table.iter().context("redb iter table")? {
            let (key_guard, value_guard) = row.context("redb iter row")?;
            let key = key_guard.value();
            if key.starts_with(prefix) {
                rows.push((key.to_string(), value_guard.value().to_vec()));
            }
        }
        rows.sort_by(|a, b| a.0.cmp(&b.0));
        Ok(rows)
    }
}

#[derive(Clone)]
pub struct StateStore {
    backend: Arc<dyn KvBackend>,
}

impl fmt::Debug for StateStore {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("StateStore{backend:dyn KvBackend}")
    }
}

impl StateStore {
    pub fn new() -> Self {
        Self {
            backend: Arc::new(MemoryKvBackend::new()),
        }
    }

    pub fn with_redb(path: impl AsRef<Path>) -> anyhow::Result<Self> {
        let backend = RedbKvBackend::open(path)?;
        Ok(Self {
            backend: Arc::new(backend),
        })
    }

    pub fn with_backend(backend: Arc<dyn KvBackend>) -> Self {
        Self { backend }
    }

    pub async fn seed_demo_data(&self) -> anyhow::Result<()> {
        if self.has_prefix(PREFIX_WORKLOADS)? {
            return Ok(());
        }

        let now = Utc::now();
        self.upsert_workload(WorkloadSummary {
            id: String::from("wk-nginx-01"),
            name: String::from("nginx-demo"),
            runtime: String::from("docker"),
            status: String::from("running"),
            node_id: String::from("node-1"),
            created_at: now,
        })
        .await?;

        self.upsert_endpoint(EndpointRecord {
            workload_id: String::from("wk-nginx-01"),
            node_id: String::from("node-1"),
            protocol: String::from("http"),
            address: String::from("10.88.0.10:80"),
        })
        .await?;

        self.upsert_route(RouteRecord {
            id: String::from("route-nginx-http"),
            protocol: String::from("http"),
            host: String::from("nginx.local"),
            path_prefix: String::from("/"),
            listen_port: 8088,
            backend: String::from("10.88.0.10:80"),
            enabled: true,
        })
        .await?;

        self.upsert_dns_record(DnsRecord {
            domain: String::from("nginx.local"),
            values: vec![String::from("10.88.0.10")],
            ttl: 60,
        })
        .await?;

        Ok(())
    }

    pub async fn upsert_workload(&self, item: WorkloadSummary) -> anyhow::Result<()> {
        let key = format!("{PREFIX_WORKLOADS}{}", item.id);
        self.put_json(&key, &item)
    }

    pub async fn upsert_endpoint(&self, item: EndpointRecord) -> anyhow::Result<()> {
        let key = format!(
            "{PREFIX_ENDPOINTS}{}|{}|{}",
            item.workload_id, item.node_id, item.protocol
        );
        self.put_json(&key, &item)
    }

    pub async fn upsert_route(&self, item: RouteRecord) -> anyhow::Result<()> {
        let key = format!("{PREFIX_ROUTES}{}", item.id);
        self.put_json(&key, &item)
    }

    pub async fn upsert_dns_record(&self, item: DnsRecord) -> anyhow::Result<()> {
        let key = format!("{PREFIX_DNS}{}", item.domain);
        self.put_json(&key, &item)
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
        match self.load_list::<WorkloadSummary>(PREFIX_WORKLOADS) {
            Ok(mut data) => {
                data.sort_by(|a, b| a.name.cmp(&b.name));
                data
            }
            Err(err) => {
                warn!(target: "warden::store", error = %err, "list workloads failed");
                Vec::new()
            }
        }
    }

    pub async fn list_endpoints(&self) -> Vec<EndpointRecord> {
        match self.load_list::<EndpointRecord>(PREFIX_ENDPOINTS) {
            Ok(mut data) => {
                data.sort_by(|a, b| a.workload_id.cmp(&b.workload_id));
                data
            }
            Err(err) => {
                warn!(target: "warden::store", error = %err, "list endpoints failed");
                Vec::new()
            }
        }
    }

    pub async fn list_routes(&self) -> Vec<RouteRecord> {
        match self.load_list::<RouteRecord>(PREFIX_ROUTES) {
            Ok(mut data) => {
                data.sort_by(|a, b| a.id.cmp(&b.id));
                data
            }
            Err(err) => {
                warn!(target: "warden::store", error = %err, "list routes failed");
                Vec::new()
            }
        }
    }

    pub async fn list_dns_records(&self) -> Vec<DnsRecord> {
        match self.load_list::<DnsRecord>(PREFIX_DNS) {
            Ok(mut data) => {
                data.sort_by(|a, b| a.domain.cmp(&b.domain));
                data
            }
            Err(err) => {
                warn!(target: "warden::store", error = %err, "list dns records failed");
                Vec::new()
            }
        }
    }

    fn put_json<T: Serialize>(&self, key: &str, value: &T) -> anyhow::Result<()> {
        let payload = serde_json::to_vec(value).with_context(|| format!("serialize key {key}"))?;
        self.backend.put(key, &payload)
    }

    fn load_list<T: DeserializeOwned>(&self, prefix: &str) -> anyhow::Result<Vec<T>> {
        let rows = self.backend.scan_prefix(prefix)?;
        rows.into_iter()
            .map(|(_, payload)| serde_json::from_slice(&payload).context("deserialize store value"))
            .collect()
    }

    fn has_prefix(&self, prefix: &str) -> anyhow::Result<bool> {
        let rows = self.backend.scan_prefix(prefix)?;
        Ok(!rows.is_empty())
    }
}

impl Default for StateStore {
    fn default() -> Self {
        Self::new()
    }
}

pub fn new_store(engine: &str, path: &str) -> anyhow::Result<StateStore> {
    match engine.trim().to_ascii_lowercase().as_str() {
        "memory" | "mem" => Ok(StateStore::new()),
        "redb" => {
            if path.trim().is_empty() {
                bail!("store.path is required when store.engine=redb");
            }
            StateStore::with_redb(path)
        }
        other => bail!("unsupported store engine: {other} (expected memory|redb)"),
    }
}
