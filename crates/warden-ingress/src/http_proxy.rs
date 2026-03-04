use crate::state::IngressInner;
use crate::util::{
  ensure_path, host_match, http_cache_key, normalize_host_with_port, response_with_status,
  route_protocol_match,
};
use axum::body::{Body, to_bytes};
use axum::extract::State;
use axum::http::header::HOST;
use axum::http::{Request, Response, StatusCode};
use std::sync::Arc;

pub(crate) async fn proxy_http_handler(
  State(inner): State<Arc<IngressInner>>,
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

  if let Some(backend) = inner.http_cache.get(&cache_key) {
    return reverse_proxy_http(&inner, req, backend).await;
  }

  let backend = match resolve_http_backend(&inner, &normalized_host, &path).await {
    Some(item) => item,
    None => return response_with_status(StatusCode::NOT_FOUND, "route not found"),
  };

  inner.http_cache.insert(cache_key, backend.clone());
  reverse_proxy_http(&inner, req, backend).await
}

async fn reverse_proxy_http(
  inner: &Arc<IngressInner>,
  req: Request<Body>,
  backend: String,
) -> Response<Body> {
  let target = if let Some(value) = backend.strip_prefix("http://") {
    value.to_string()
  } else if backend.starts_with("https://") {
    return response_with_status(
      StatusCode::BAD_GATEWAY,
      "https backend is not supported yet",
    );
  } else {
    backend
  };

  let path_and_query = req
    .uri()
    .path_and_query()
    .map(|v| v.as_str().to_string())
    .unwrap_or_else(|| String::from("/"));
  let target_url = format!("http://{target}{path_and_query}");
  let (parts, body) = req.into_parts();
  let body_bytes = match to_bytes(body, inner.options.max_request_body).await {
    Ok(payload) => payload,
    Err(err) => {
      return response_with_status(
        StatusCode::BAD_GATEWAY,
        &format!("proxy read body error: {err}"),
      );
    }
  };

  let mut outbound = inner
    .http_client
    .request(parts.method.clone(), target_url)
    .body(body_bytes.to_vec());
  for (name, value) in &parts.headers {
    if *name != HOST {
      outbound = outbound.header(name, value);
    }
  }

  let upstream = match outbound.send().await {
    Ok(resp) => resp,
    Err(err) => {
      return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
    }
  };
  let status = upstream.status();
  let headers = upstream.headers().clone();
  let payload = match upstream.bytes().await {
    Ok(data) => data,
    Err(err) => {
      return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
    }
  };

  let mut response = Response::new(Body::from(payload));
  *response.status_mut() = status;
  for (name, value) in &headers {
    response.headers_mut().insert(name, value.clone());
  }
  response
}

async fn resolve_http_backend(inner: &Arc<IngressInner>, host: &str, path: &str) -> Option<String> {
  let routes = inner.registry.list_routes().await;
  let normalized_host = normalize_host_with_port(host);
  let normalized_path = ensure_path(path);

  let mut selected_backend = None;
  let mut longest = 0usize;
  for route in routes {
    if !route.enabled
      || !route_protocol_match(&route, "http")
      || !host_match(&route.host, &normalized_host)
    {
      continue;
    }
    let prefix = ensure_path(&route.path_prefix);
    if !normalized_path.starts_with(&prefix) || prefix.len() <= longest {
      continue;
    }
    if route.backend.trim().is_empty() {
      continue;
    }
    longest = prefix.len();
    selected_backend = Some(route.backend);
  }
  selected_backend
}
