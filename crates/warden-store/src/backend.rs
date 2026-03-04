use crate::StateStore;
use anyhow::bail;
use redb::TableDefinition;

pub(crate) const PREFIX_WORKLOADS: &str = "workloads/";
pub(crate) const PREFIX_ENDPOINTS: &str = "endpoints/";
pub(crate) const PREFIX_ROUTES: &str = "routes/";
pub(crate) const PREFIX_DNS: &str = "dns/";
pub(crate) const KV_TABLE: TableDefinition<&str, &[u8]> = TableDefinition::new("warden_kv");

pub trait KvBackend: Send + Sync {
  fn get(&self, key: &str) -> anyhow::Result<Option<Vec<u8>>>;
  fn put(&self, key: &str, value: &[u8]) -> anyhow::Result<()>;
  fn delete(&self, key: &str) -> anyhow::Result<()>;
  fn scan_prefix(&self, prefix: &str) -> anyhow::Result<Vec<(String, Vec<u8>)>>;
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

pub(crate) fn poisoned_lock_err() -> anyhow::Error {
  anyhow::anyhow!("memory kv lock poisoned")
}
