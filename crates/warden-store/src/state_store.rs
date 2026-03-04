use crate::{KvBackend, MemoryKvBackend, RedbKvBackend};
use anyhow::Context;
use serde::{Serialize, de::DeserializeOwned};
use std::fmt;
use std::path::Path;
use std::sync::Arc;

#[derive(Clone)]
pub struct StateStore {
  pub(crate) backend: Arc<dyn KvBackend>,
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

  pub(crate) fn put_json<T: Serialize>(&self, key: &str, value: &T) -> anyhow::Result<()> {
    let payload = serde_json::to_vec(value).with_context(|| format!("serialize key {key}"))?;
    self.backend.put(key, &payload)
  }

  pub(crate) fn load_list<T: DeserializeOwned>(&self, prefix: &str) -> anyhow::Result<Vec<T>> {
    let rows = self.backend.scan_prefix(prefix)?;
    rows
      .into_iter()
      .map(|(_, payload)| serde_json::from_slice(&payload).context("deserialize store value"))
      .collect()
  }

  pub(crate) fn has_prefix(&self, prefix: &str) -> anyhow::Result<bool> {
    Ok(!self.backend.scan_prefix(prefix)?.is_empty())
  }
}

impl Default for StateStore {
  fn default() -> Self {
    Self::new()
  }
}
