use axum::Router;
use axum::body::Body;
use axum::extract::State;
use axum::http::header::HOST;
use axum::http::{Request, Response, StatusCode};
use axum::routing::any;
use moka::sync::Cache;
use std::sync::Arc;
use tokio::sync::RwLock;
use warden_ingress_proxy::{proxy_http_request, response_with_status};
use warden_ingress_resolver::resolve_http_backend;
use warden_ingress_types::{
  IngressOptions, IngressRouteSnapshot, http_cache_key, normalize_host_with_port,
};

#[derive(Clone)]
pub struct HttpIngressState {
  snapshot: Arc<RwLock<IngressRouteSnapshot>>,
  http_client: reqwest::Client,
  http_cache: Cache<String, String>,
  max_request_body: usize,
}

impl HttpIngressState {
  pub fn new(options: &IngressOptions) -> Self {
    let cache = Cache::builder()
      .max_capacity(options.http_cache_capacity)
      .time_to_live(options.http_cache_ttl)
      .build();

    let http_client = reqwest::Client::builder()
      .timeout(options.http_proxy_timeout)
      .build()
      .unwrap_or_else(|_| reqwest::Client::new());

    Self {
      snapshot: Arc::new(RwLock::new(IngressRouteSnapshot::default())),
      http_client,
      http_cache: cache,
      max_request_body: options.max_request_body,
    }
  }

  pub async fn replace_snapshot(&self, snapshot: IngressRouteSnapshot) {
    *self.snapshot.write().await = snapshot;
    self.http_cache.invalidate_all();
  }
}

pub fn build_router(state: HttpIngressState) -> Router {
  Router::new()
    .fallback(any(proxy_http_handler))
    .with_state(state)
}

async fn proxy_http_handler(
  State(state): State<HttpIngressState>,
  req: Request<Body>,
) -> Response<Body> {
  let host = req
    .headers()
    .get(HOST)
    .and_then(|v| v.to_str().ok())
    .unwrap_or_default();
  let normalized_host = normalize_host_with_port(host);
  let path = req.uri().path().to_string();
  let cache_key = http_cache_key(&normalized_host, &path);

  if let Some(backend) = state.http_cache.get(&cache_key) {
    return proxy_http_request(&state.http_client, state.max_request_body, req, backend).await;
  }

  let backend = {
    let snapshot = state.snapshot.read().await;
    resolve_http_backend(&snapshot, &normalized_host, &path)
  };
  let backend = match backend {
    Some(item) => item,
    None => return response_with_status(StatusCode::NOT_FOUND, "route not found"),
  };

  state.http_cache.insert(cache_key, backend.clone());
  proxy_http_request(&state.http_client, state.max_request_body, req, backend).await
}
