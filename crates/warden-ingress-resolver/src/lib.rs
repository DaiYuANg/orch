use warden_ingress_types::{
  HttpRoute, IngressBackendBinding, IngressControlPlane, IngressControlRoute, IngressRouteSnapshot,
  StreamRoute, ensure_path, host_match, normalize_host_with_port, route_protocol_match,
};
use warden_registry::RegistryService;
use warden_types::{EndpointRecord, RouteRecord};

pub async fn load_snapshot(registry: &RegistryService) -> IngressRouteSnapshot {
  let routes = registry.list_routes().await;
  let endpoints = registry.list_endpoints().await;
  let control_plane = compile_control_plane(&routes, &endpoints);
  compile_snapshot(&control_plane, &endpoints)
}

pub fn compile_control_plane(
  routes: &[RouteRecord],
  endpoints: &[EndpointRecord],
) -> IngressControlPlane {
  let mut control_plane = IngressControlPlane::default();

  for route in routes {
    if !route.enabled || !route_has_backend_binding(route) {
      continue;
    }

    control_plane.routes.push(IngressControlRoute {
      id: route.id.clone(),
      protocol: route.protocol.clone(),
      host: normalize_host_with_port(&route.host),
      path_prefix: ensure_path(&route.path_prefix),
      listen_port: route.listen_port,
      binding: resolve_route_binding(route, endpoints),
      backend_hint: if route.backend.trim().is_empty() {
        None
      } else {
        Some(route.backend.clone())
      },
      enabled: route.enabled,
    });
  }

  control_plane
}

pub fn compile_snapshot(
  control_plane: &IngressControlPlane,
  endpoints: &[EndpointRecord],
) -> IngressRouteSnapshot {
  let mut snapshot = IngressRouteSnapshot::default();

  for route in &control_plane.routes {
    if !route.enabled {
      continue;
    }

    if route_protocol_match(&route.protocol, "http") {
      let eligible_backends = resolve_eligible_backends(route, endpoints);
      snapshot.http_routes.push(HttpRoute {
        id: route.id.clone(),
        host: route.host.clone(),
        path_prefix: route.path_prefix.clone(),
        binding: route.binding.clone(),
        backend: route.backend_hint.clone().unwrap_or_default(),
        eligible_backends,
      });
      continue;
    }

    if route.listen_port == 0 {
      continue;
    }

    let stream = StreamRoute {
      listen_port: route.listen_port,
      binding: route.binding.clone(),
      backend: select_stream_backend(route, endpoints),
    };

    if route_protocol_match(&route.protocol, "tcp") {
      snapshot.tcp_routes.push(stream);
    } else if route_protocol_match(&route.protocol, "udp") {
      snapshot.udp_routes.push(stream);
    }
  }

  snapshot
}

pub fn resolve_http_backend(
  snapshot: &IngressRouteSnapshot,
  host: &str,
  path: &str,
) -> Option<String> {
  let normalized_host = normalize_host_with_port(host);
  let normalized_path = ensure_path(path);

  let mut selected_backend = None;
  let mut longest = 0usize;
  for route in &snapshot.http_routes {
    if !host_match(&route.host, &normalized_host) {
      continue;
    }
    let prefix = route.path_prefix.as_str();
    if !normalized_path.starts_with(prefix) || prefix.len() <= longest {
      continue;
    }
    longest = prefix.len();
    selected_backend = Some(select_http_backend(route));
  }
  selected_backend
}

fn select_http_backend(route: &HttpRoute) -> String {
  route
    .eligible_backends
    .first()
    .cloned()
    .unwrap_or_else(|| route.backend.clone())
}

fn select_stream_backend(route: &IngressControlRoute, endpoints: &[EndpointRecord]) -> String {
  resolve_eligible_backends(route, endpoints)
    .into_iter()
    .next()
    .unwrap_or_else(|| route.backend_hint.clone().unwrap_or_default())
}

fn resolve_eligible_backends(
  route: &IngressControlRoute,
  endpoints: &[EndpointRecord],
) -> Vec<String> {
  let route_protocol = route.protocol.trim();
  let route_backend = route.backend_hint.as_deref().unwrap_or("").trim();

  let explicit_workload_id = route
    .binding
    .as_ref()
    .map(|value| value.workload_id.as_str());
  let explicit_endpoint_name = route
    .binding
    .as_ref()
    .and_then(|value| value.endpoint_name.as_deref());

  let inferred_workload_id = endpoints
    .iter()
    .find(|endpoint| {
      endpoint.protocol.eq_ignore_ascii_case(route_protocol)
        && endpoint.address.trim().eq_ignore_ascii_case(route_backend)
    })
    .map(|endpoint| endpoint.workload_id.as_str());

  let mut backends = endpoints
    .iter()
    .filter(|endpoint| endpoint.protocol.eq_ignore_ascii_case(route_protocol))
    .filter(|endpoint| endpoint.healthy && endpoint.ready)
    .filter(
      |endpoint| match explicit_workload_id.or(inferred_workload_id) {
        Some(workload_id) => endpoint.workload_id == workload_id,
        None => endpoint.address.trim().eq_ignore_ascii_case(route_backend),
      },
    )
    .filter(|endpoint| match explicit_endpoint_name {
      Some(endpoint_name) => endpoint.endpoint_name == endpoint_name,
      None => true,
    })
    .map(|endpoint| endpoint.address.clone())
    .collect::<Vec<_>>();

  backends.sort();
  backends.dedup();
  backends
}

fn resolve_route_binding(
  route: &RouteRecord,
  endpoints: &[EndpointRecord],
) -> Option<IngressBackendBinding> {
  if let Some(workload_id) = route.backend_workload_id.as_ref() {
    return Some(IngressBackendBinding {
      workload_id: workload_id.clone(),
      endpoint_name: route.backend_endpoint_name.clone(),
    });
  }

  let route_protocol = route.protocol.trim();
  let route_backend = route.backend.trim();
  let endpoint = endpoints.iter().find(|endpoint| {
    endpoint.protocol.eq_ignore_ascii_case(route_protocol)
      && endpoint.address.trim().eq_ignore_ascii_case(route_backend)
  })?;

  Some(IngressBackendBinding {
    workload_id: endpoint.workload_id.clone(),
    endpoint_name: Some(endpoint.endpoint_name.clone()),
  })
}

fn route_has_backend_binding(route: &RouteRecord) -> bool {
  !route.backend.trim().is_empty() || route.backend_workload_id.is_some()
}

#[cfg(test)]
mod tests {
  use super::{compile_control_plane, compile_snapshot, resolve_http_backend};
  use chrono::Utc;
  use warden_types::{EndpointRecord, RouteRecord};

  fn route(
    id: &str,
    protocol: &str,
    host: &str,
    path_prefix: &str,
    listen_port: u16,
    backend: &str,
  ) -> RouteRecord {
    RouteRecord {
      id: id.to_string(),
      protocol: protocol.to_string(),
      host: host.to_string(),
      path_prefix: path_prefix.to_string(),
      listen_port,
      backend: backend.to_string(),
      backend_workload_id: None,
      backend_endpoint_name: None,
      enabled: true,
    }
  }

  fn route_with_endpoint_binding(
    id: &str,
    protocol: &str,
    host: &str,
    path_prefix: &str,
    listen_port: u16,
    backend: &str,
    workload_id: &str,
    endpoint_name: &str,
  ) -> RouteRecord {
    RouteRecord {
      id: id.to_string(),
      protocol: protocol.to_string(),
      host: host.to_string(),
      path_prefix: path_prefix.to_string(),
      listen_port,
      backend: backend.to_string(),
      backend_workload_id: Some(workload_id.to_string()),
      backend_endpoint_name: Some(endpoint_name.to_string()),
      enabled: true,
    }
  }

  fn endpoint(
    workload_id: &str,
    protocol: &str,
    address: &str,
    healthy: bool,
    ready: bool,
  ) -> EndpointRecord {
    EndpointRecord {
      workload_id: workload_id.to_string(),
      node_id: String::from("node-1"),
      endpoint_name: String::from("http"),
      protocol: protocol.to_string(),
      address: address.to_string(),
      healthy,
      ready,
      updated_at: Utc::now(),
    }
  }

  #[test]
  fn compile_snapshot_splits_routes_by_protocol() {
    let routes = vec![
      route("http-1", "http", "api.example.com", "/", 0, "10.0.0.1:8080"),
      route("tcp-1", "tcp", "", "", 5432, "10.0.0.2:5432"),
      route("udp-1", "udp", "", "", 5353, "10.0.0.3:5353"),
    ];
    let control_plane = compile_control_plane(&routes, &[]);
    let snapshot = compile_snapshot(&control_plane, &[]);

    assert_eq!(snapshot.http_routes.len(), 1);
    assert_eq!(snapshot.tcp_routes.len(), 1);
    assert_eq!(snapshot.udp_routes.len(), 1);
    assert!(snapshot.http_routes[0].binding.is_none());
  }

  #[test]
  fn resolve_http_backend_uses_longest_path_prefix() {
    let routes = vec![
      route("r1", "http", "api.example.com", "/", 0, "10.0.0.1:8080"),
      route("r2", "http", "api.example.com", "/v1", 0, "10.0.0.2:8080"),
    ];
    let control_plane = compile_control_plane(&routes, &[]);
    let snapshot = compile_snapshot(&control_plane, &[]);

    let backend = resolve_http_backend(&snapshot, "api.example.com", "/v1/users");
    assert_eq!(backend.as_deref(), Some("10.0.0.2:8080"));
  }

  #[test]
  fn resolve_http_backend_prefers_healthy_ready_endpoint_candidates() {
    let routes = vec![route(
      "r1",
      "http",
      "api.example.com",
      "/",
      0,
      "10.0.0.1:8080",
    )];
    let endpoints = vec![
      endpoint("wk-api", "http", "10.0.0.1:8080", false, true),
      endpoint("wk-api", "http", "10.0.0.2:8080", true, true),
      endpoint("wk-api", "http", "10.0.0.3:8080", true, false),
    ];
    let control_plane = compile_control_plane(&routes, &endpoints);
    let snapshot = compile_snapshot(&control_plane, &endpoints);

    let backend = resolve_http_backend(&snapshot, "api.example.com", "/");
    assert_eq!(backend.as_deref(), Some("10.0.0.2:8080"));
  }

  #[test]
  fn resolve_http_backend_prefers_explicit_endpoint_binding_over_backend_string_inference() {
    let routes = vec![route_with_endpoint_binding(
      "r1",
      "http",
      "api.example.com",
      "/",
      0,
      "stale-backend:8080",
      "wk-api",
      "http",
    )];
    let endpoints = vec![
      endpoint("wk-api", "http", "10.0.0.2:8080", true, true),
      endpoint("wk-other", "http", "10.0.0.9:8080", true, true),
    ];
    let control_plane = compile_control_plane(&routes, &endpoints);
    let snapshot = compile_snapshot(&control_plane, &endpoints);

    let backend = resolve_http_backend(&snapshot, "api.example.com", "/");
    assert_eq!(backend.as_deref(), Some("10.0.0.2:8080"));
    assert_eq!(
      snapshot.http_routes[0]
        .binding
        .as_ref()
        .map(|value| value.workload_id.as_str()),
      Some("wk-api")
    );
    assert_eq!(
      snapshot.http_routes[0]
        .binding
        .as_ref()
        .and_then(|value| value.endpoint_name.as_deref()),
      Some("http")
    );
  }

  #[test]
  fn compile_snapshot_can_infer_binding_from_backend_address() {
    let routes = vec![route(
      "r1",
      "http",
      "api.example.com",
      "/",
      0,
      "10.0.0.1:8080",
    )];
    let endpoints = vec![endpoint("wk-api", "http", "10.0.0.1:8080", true, true)];
    let control_plane = compile_control_plane(&routes, &endpoints);
    let snapshot = compile_snapshot(&control_plane, &endpoints);

    assert_eq!(
      snapshot.http_routes[0]
        .binding
        .as_ref()
        .map(|value| value.workload_id.as_str()),
      Some("wk-api")
    );
    assert_eq!(
      snapshot.http_routes[0]
        .binding
        .as_ref()
        .and_then(|value| value.endpoint_name.as_deref()),
      Some("http")
    );
  }

  #[test]
  fn compile_control_plane_keeps_binding_and_backend_hint_separate() {
    let routes = vec![route_with_endpoint_binding(
      "r1",
      "http",
      "api.example.com",
      "/",
      0,
      "10.0.0.1:8080",
      "wk-api",
      "http",
    )];
    let control_plane = compile_control_plane(&routes, &[]);

    assert_eq!(control_plane.routes.len(), 1);
    assert_eq!(
      control_plane.routes[0].backend_hint.as_deref(),
      Some("10.0.0.1:8080")
    );
    assert_eq!(
      control_plane.routes[0]
        .binding
        .as_ref()
        .map(|value| value.workload_id.as_str()),
      Some("wk-api")
    );
  }
}
