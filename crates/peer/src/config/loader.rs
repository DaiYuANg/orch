use crate::config::WardenConfig;
use figment::providers::{Env, Format, Serialized, Toml};
use figment::Figment;

pub fn load_config(path: &str) -> WardenConfig {
  Figment::from(Serialized::defaults(WardenConfig::default()))
    .merge(Toml::file(path))
    .merge(Env::prefixed("WARDEN_").split("_")) // 支持 WARDEN_NODE_NAME 等环境变量覆盖
    .extract()
    .expect("Failed to load Warden config")
}
