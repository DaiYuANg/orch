use tracing_subscriber::EnvFilter;

pub fn init(cfg: &warden_config::LoggerConfig) {
  let filter = EnvFilter::try_new(cfg.level.clone()).unwrap_or_else(|_| EnvFilter::new("info"));
  let _ = tracing_subscriber::fmt()
    .with_env_filter(filter)
    .with_target(true)
    .with_thread_names(true)
    .try_init();
}
