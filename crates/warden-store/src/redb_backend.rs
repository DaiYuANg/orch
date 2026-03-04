use crate::backend::{KV_TABLE, KvBackend};
use anyhow::Context;
use redb::{Database, ReadableDatabase, ReadableTable};
use std::path::Path;

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
    Ok(maybe_value.map(|value| value.value().to_vec()))
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
