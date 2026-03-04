use crate::state::IngressInner;
use axum::body::Body;
use axum::http::{Response, StatusCode};
use std::sync::Arc;
use tokio::sync::watch;
use warden_types::RouteRecord;

pub(crate) fn route_protocol_match(route: &RouteRecord, protocol: &str) -> bool {
  route.protocol.trim().eq_ignore_ascii_case(protocol)
}

pub(crate) fn host_match(pattern: &str, host: &str) -> bool {
  let left = normalize_host_with_port(pattern);
  let right = normalize_host_with_port(host);
  left.is_empty() || left == "*" || left == right
}

pub(crate) fn ensure_path(path: &str) -> String {
  let value = path.trim();
  if value.is_empty() {
    String::from("/")
  } else if value.starts_with('/') {
    value.to_string()
  } else {
    format!("/{value}")
  }
}

pub(crate) fn normalize_host_with_port(host: &str) -> String {
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

pub(crate) fn normalize_bind_addr(addr: &str) -> String {
  let value = addr.trim();
  if let Some(port) = value.strip_prefix(':') {
    return format!("0.0.0.0:{port}");
  }
  value.to_string()
}

pub(crate) fn response_with_status(status: StatusCode, body: &str) -> Response<Body> {
  let mut response = Response::new(Body::from(body.to_string()));
  *response.status_mut() = status;
  response
}

pub(crate) fn http_cache_key(host: &str, path: &str) -> String {
  format!("{}|{}", host.trim().to_ascii_lowercase(), path.trim())
}

pub(crate) async fn wait_for_stop(stop_rx: &mut watch::Receiver<bool>) {
  if *stop_rx.borrow() {
    return;
  }
  while stop_rx.changed().await.is_ok() {
    if *stop_rx.borrow() {
      return;
    }
  }
}

#[allow(dead_code)]
pub(crate) fn cache_len(inner: &Arc<IngressInner>) -> u64 {
  inner.http_cache.entry_count()
}
