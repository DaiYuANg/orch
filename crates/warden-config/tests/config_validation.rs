use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};
use warden_config::load;

#[test]
fn rejects_zero_timeout_values() {
  let path = write_temp_json(
    "timeout",
    r#"{
      "timeouts": {
        "ingress_http_cache_ttl_ms": 0,
        "ingress_http_proxy_timeout_ms": 1000,
        "ingress_stream_sync_interval_ms": 1000,
        "ingress_udp_backend_timeout_ms": 1000,
        "dns_record_cache_ttl_ms": 1000
      }
    }"#,
  );

  let result = load(&[path.clone()]);
  assert!(result.is_err());
  assert!(
    result
      .unwrap_err()
      .to_string()
      .contains("validate rust config")
  );
  let _ = fs::remove_file(path);
}

#[test]
fn rejects_redb_without_store_path() {
  let path = write_temp_json(
    "redb",
    r#"{
      "store": {
        "engine": "redb",
        "path": "   "
      }
    }"#,
  );

  let result = load(&[path.clone()]);
  assert!(result.is_err());
  assert!(
    result
      .unwrap_err()
      .to_string()
      .contains("validate rust config")
  );
  let _ = fs::remove_file(path);
}

#[test]
fn allows_memory_store_without_path() -> anyhow::Result<()> {
  let path = write_temp_json(
    "memory",
    r#"{
      "store": {
        "engine": "memory",
        "path": ""
      }
    }"#,
  );

  let result = load(&[path.clone()]);
  let _ = fs::remove_file(path);
  result.map(|_| ())
}

fn write_temp_json(tag: &str, content: &str) -> PathBuf {
  let stamp = SystemTime::now()
    .duration_since(UNIX_EPOCH)
    .unwrap_or_default()
    .as_nanos();
  let name = format!("warden-config-{tag}-{stamp}.json");
  let path = std::env::temp_dir().join(name);
  fs::write(&path, content).expect("write temp config file");
  path
}
