use anyhow::{anyhow, bail};
use std::path::PathBuf;

#[derive(Debug, Clone)]
pub enum Endpoint {
  Http(String),
  Unix(PathBuf),
  Npipe(String),
  Auto,
}

pub fn parse_endpoint(raw: &str) -> anyhow::Result<Endpoint> {
  let trimmed = raw.trim();
  if trimmed.is_empty() || trimmed.eq_ignore_ascii_case("auto") {
    return Ok(Endpoint::Auto);
  }
  if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
    return Ok(Endpoint::Http(trimmed.trim_end_matches('/').to_string()));
  }
  if let Some(rest) = trimmed.strip_prefix("unix://") {
    let path = rest.trim();
    if path.is_empty() {
      bail!("invalid unix endpoint: empty socket path");
    }
    return Ok(Endpoint::Unix(PathBuf::from(path)));
  }
  if let Some(rest) = trimmed.strip_prefix("npipe://") {
    let pipe = normalize_npipe_path(rest);
    if pipe.is_empty() {
      bail!("invalid npipe endpoint: empty pipe path");
    }
    return Ok(Endpoint::Npipe(pipe));
  }
  Err(anyhow!(
    "unsupported endpoint {trimmed}, expected auto|unix://|npipe://|http(s)://"
  ))
}

pub(crate) fn auto_endpoints() -> Vec<Endpoint> {
  #[cfg(windows)]
  {
    vec![
      Endpoint::Npipe(String::from(r"\\.\pipe\warden")),
      Endpoint::Http(String::from("http://127.0.0.1:7443")),
    ]
  }

  #[cfg(not(windows))]
  {
    let socket = std::env::temp_dir().join("warden.sock");
    vec![
      Endpoint::Unix(socket),
      Endpoint::Http(String::from("http://127.0.0.1:7443")),
    ]
  }
}

pub(crate) fn describe_endpoint(endpoint: &Endpoint) -> String {
  match endpoint {
    Endpoint::Http(base) => format!("http({base})"),
    Endpoint::Unix(path) => format!("unix({})", path.display()),
    Endpoint::Npipe(path) => format!("npipe({path})"),
    Endpoint::Auto => String::from("auto"),
  }
}

pub(crate) fn ensure_path(path: &str) -> String {
  let p = path.trim();
  if p.is_empty() {
    String::from("/")
  } else if p.starts_with('/') {
    p.to_string()
  } else {
    format!("/{p}")
  }
}

fn normalize_npipe_path(raw: &str) -> String {
  let mut value = raw.trim().replace('/', "\\");
  if value.is_empty() {
    return value;
  }
  if value.starts_with(r"\\.\pipe\") {
    return value;
  }
  value = value.trim_start_matches('\\').to_string();
  if let Some(rest) = value.strip_prefix("pipe\\") {
    return format!(r"\\.\pipe\{}", rest.trim_start_matches('\\'));
  }
  if let Some(rest) = value.strip_prefix(".\\pipe\\") {
    return format!(r"\\.\pipe\{}", rest.trim_start_matches('\\'));
  }
  format!(r"\\.\pipe\{}", value)
}
