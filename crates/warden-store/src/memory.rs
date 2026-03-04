use crate::backend::{KvBackend, poisoned_lock_err};
use std::collections::HashMap;
use std::sync::RwLock;

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
    let guard = self.inner.read().map_err(|_| poisoned_lock_err())?;
    Ok(guard.get(key).cloned())
  }

  fn put(&self, key: &str, value: &[u8]) -> anyhow::Result<()> {
    let mut guard = self.inner.write().map_err(|_| poisoned_lock_err())?;
    guard.insert(key.to_string(), value.to_vec());
    Ok(())
  }

  fn delete(&self, key: &str) -> anyhow::Result<()> {
    let mut guard = self.inner.write().map_err(|_| poisoned_lock_err())?;
    let _ = guard.remove(key);
    Ok(())
  }

  fn scan_prefix(&self, prefix: &str) -> anyhow::Result<Vec<(String, Vec<u8>)>> {
    let guard = self.inner.read().map_err(|_| poisoned_lock_err())?;
    let mut rows: Vec<(String, Vec<u8>)> = guard
      .iter()
      .filter(|(key, _)| key.starts_with(prefix))
      .map(|(key, value)| (key.clone(), value.clone()))
      .collect();
    rows.sort_by(|a, b| a.0.cmp(&b.0));
    Ok(rows)
  }
}
