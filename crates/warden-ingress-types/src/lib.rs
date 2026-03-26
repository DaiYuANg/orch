use std::time::Duration;

pub const DEFAULT_HTTP_CACHE_CAPACITY: u64 = 2048;
pub const DEFAULT_MAX_REQUEST_BODY: usize = 10 * 1024 * 1024;

#[derive(Debug, Clone)]
pub struct IngressOptions {
  pub http_cache_capacity: u64,
  pub http_cache_ttl: Duration,
  pub http_proxy_timeout: Duration,
  pub max_request_body: usize,
  pub stream_sync_interval: Duration,
  pub udp_backend_timeout: Duration,
}

impl Default for IngressOptions {
  fn default() -> Self {
    Self {
      http_cache_capacity: DEFAULT_HTTP_CACHE_CAPACITY,
      http_cache_ttl: Duration::from_secs(2),
      http_proxy_timeout: Duration::from_secs(10),
      max_request_body: DEFAULT_MAX_REQUEST_BODY,
      stream_sync_interval: Duration::from_secs(3),
      udp_backend_timeout: Duration::from_secs(3),
    }
  }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StreamRoute {
  pub listen_port: u16,
  pub binding: Option<IngressBackendBinding>,
  pub backend: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HttpRoute {
  pub id: String,
  pub host: String,
  pub path_prefix: String,
  pub binding: Option<IngressBackendBinding>,
  pub backend: String,
  pub eligible_backends: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IngressBackendBinding {
  pub workload_id: String,
  pub endpoint_name: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IngressControlRoute {
  pub id: String,
  pub protocol: String,
  pub host: String,
  pub path_prefix: String,
  pub listen_port: u16,
  pub binding: Option<IngressBackendBinding>,
  pub backend_hint: Option<String>,
  pub enabled: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct IngressControlPlane {
  pub routes: Vec<IngressControlRoute>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct IngressRouteSnapshot {
  pub http_routes: Vec<HttpRoute>,
  pub tcp_routes: Vec<StreamRoute>,
  pub udp_routes: Vec<StreamRoute>,
}

pub fn route_protocol_match(route_protocol: &str, protocol: &str) -> bool {
  route_protocol.trim().eq_ignore_ascii_case(protocol)
}

pub fn host_match(pattern: &str, host: &str) -> bool {
  let left = normalize_host_with_port(pattern);
  let right = normalize_host_with_port(host);
  left.is_empty() || left == "*" || left == right
}

pub fn ensure_path(path: &str) -> String {
  let value = path.trim();
  if value.is_empty() {
    String::from("/")
  } else if value.starts_with('/') {
    value.to_string()
  } else {
    format!("/{value}")
  }
}

pub fn normalize_host_with_port(host: &str) -> String {
  let raw = host.trim().to_ascii_lowercase();
  if raw.is_empty() {
    return raw;
  }
  if raw.starts_with('[') && raw.ends_with(']') {
    return raw
      .trim_start_matches('[')
      .trim_end_matches(']')
      .to_string();
  }
  if let Some((name, port)) = raw.rsplit_once(':') {
    if port.chars().all(|ch| ch.is_ascii_digit()) {
      return name.to_string();
    }
  }
  raw
}

pub fn normalize_bind_addr(addr: &str) -> String {
  let value = addr.trim();
  if let Some(port) = value.strip_prefix(':') {
    return format!("0.0.0.0:{port}");
  }
  value.to_string()
}

pub fn http_cache_key(host: &str, path: &str) -> String {
  format!("{}|{}", host.trim().to_ascii_lowercase(), path.trim())
}
