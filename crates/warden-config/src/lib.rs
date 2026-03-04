mod validate;

use anyhow::Context;
use figment::{
  Figment,
  providers::{Env, Format, Json, Serialized, Toml, Yaml},
};
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};
use validator::Validate;

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
#[validate(schema(function = "crate::validate::validate_config_schema"))]
pub struct Config {
  #[validate(nested)]
  pub http: HttpConfig,
  #[validate(nested)]
  pub network: NetworkConfig,
  #[validate(nested)]
  pub timeouts: TimeoutConfig,
  #[validate(nested)]
  pub logger: LoggerConfig,
  #[validate(nested)]
  pub store: StoreConfig,
  #[validate(nested)]
  pub raft: RaftConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct HttpConfig {
  #[validate(range(min = 1, max = 65535))]
  pub port: u16,
  pub unix_socket: String,
  pub named_pipe: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct NetworkConfig {
  #[validate(length(min = 1))]
  pub dns_listen: String,
  #[validate(length(min = 1))]
  pub ingress_http_listen: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct TimeoutConfig {
  #[validate(range(min = 1))]
  pub ingress_http_cache_ttl_ms: u64,
  #[validate(range(min = 1))]
  pub ingress_http_proxy_timeout_ms: u64,
  #[validate(range(min = 1))]
  pub ingress_stream_sync_interval_ms: u64,
  #[validate(range(min = 1))]
  pub ingress_udp_backend_timeout_ms: u64,
  #[validate(range(min = 1))]
  pub dns_record_cache_ttl_ms: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct LoggerConfig {
  #[validate(custom(function = "crate::validate::validate_logger_level"))]
  pub level: String,
  pub console: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct StoreConfig {
  #[validate(custom(function = "crate::validate::validate_store_engine"))]
  pub engine: String,
  pub path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Validate)]
pub struct RaftConfig {
  pub enable: bool,
  #[validate(range(min = 1))]
  pub node_id: u64,
  #[validate(length(min = 1))]
  pub bind_addr: String,
}

impl Default for Config {
  fn default() -> Self {
    Self {
      http: HttpConfig {
        port: 7443,
        unix_socket: std::env::temp_dir()
          .join("warden.sock")
          .to_string_lossy()
          .to_string(),
        named_pipe: if cfg!(windows) {
          String::from(r"\\.\pipe\warden")
        } else {
          String::new()
        },
      },
      network: NetworkConfig {
        dns_listen: String::from(":1053"),
        ingress_http_listen: String::from(":8088"),
      },
      timeouts: TimeoutConfig {
        ingress_http_cache_ttl_ms: 2_000,
        ingress_http_proxy_timeout_ms: 10_000,
        ingress_stream_sync_interval_ms: 3_000,
        ingress_udp_backend_timeout_ms: 3_000,
        dns_record_cache_ttl_ms: 2_000,
      },
      logger: LoggerConfig {
        level: String::from("info"),
        console: true,
      },
      store: StoreConfig {
        engine: String::from("redb"),
        path: std::env::temp_dir()
          .join("warden.redb")
          .to_string_lossy()
          .to_string(),
      },
      raft: RaftConfig {
        enable: false,
        node_id: 1,
        bind_addr: String::from("127.0.0.1:12000"),
      },
    }
  }
}

pub fn load(files: &[PathBuf]) -> anyhow::Result<Config> {
  let mut figment = Figment::from(Serialized::defaults(Config::default()));

  for path in files {
    figment = merge_file_provider(figment, path)?;
  }

  figment = figment
    .merge(Env::prefixed("WARDEN__").split("__"))
    .merge(Env::prefixed("WARDEN_").split("__"));

  let cfg: Config = figment
    .extract()
    .context("extract rust config with figment")?;
  cfg.validate().context("validate rust config")?;
  Ok(cfg)
}

pub fn parse_conf_args(items: &[String]) -> Vec<PathBuf> {
  items
    .iter()
    .map(String::as_str)
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .map(Path::new)
    .map(Path::to_path_buf)
    .collect()
}

fn merge_file_provider(figment: Figment, path: &Path) -> anyhow::Result<Figment> {
  let ext = path
    .extension()
    .and_then(|v| v.to_str())
    .map(str::to_ascii_lowercase)
    .unwrap_or_default();

  let merged = match ext.as_str() {
    "yaml" | "yml" => figment.merge(Yaml::file(path)),
    "toml" => figment.merge(Toml::file(path)),
    "json" => figment.merge(Json::file(path)),
    _ => {
      anyhow::bail!(
        "unsupported config file extension for {} (expected .yaml/.yml/.toml/.json)",
        path.display()
      )
    }
  };
  Ok(merged)
}
