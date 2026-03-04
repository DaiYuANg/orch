use crate::Config;
use validator::ValidationError;

pub fn validate_logger_level(level: &str) -> Result<(), ValidationError> {
  let normalized = level.trim().to_ascii_lowercase();
  let ok = matches!(
    normalized.as_str(),
    "trace" | "debug" | "info" | "warn" | "error"
  );
  if ok {
    Ok(())
  } else {
    Err(ValidationError::new("invalid_logger_level"))
  }
}

pub fn validate_store_engine(engine: &str) -> Result<(), ValidationError> {
  let normalized = engine.trim().to_ascii_lowercase();
  let ok = matches!(normalized.as_str(), "memory" | "mem" | "redb");
  if ok {
    Ok(())
  } else {
    Err(ValidationError::new("invalid_store_engine"))
  }
}

pub fn validate_config_schema(cfg: &Config) -> Result<(), ValidationError> {
  if cfg.store.engine.trim().eq_ignore_ascii_case("redb") && cfg.store.path.trim().is_empty() {
    return Err(ValidationError::new("missing_store_path_for_redb"));
  }
  Ok(())
}
