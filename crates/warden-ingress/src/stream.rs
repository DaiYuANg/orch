#[path = "stream_tcp.rs"]
mod stream_tcp;
#[path = "stream_udp.rs"]
mod stream_udp;

use crate::state::IngressInner;
use crate::util::route_protocol_match;
use std::collections::HashSet;
use std::sync::Arc;
use tracing::warn;

pub(crate) async fn sync_stream_routes(inner: &Arc<IngressInner>, protocol: &str) {
  let routes = inner.registry.list_routes().await;
  let mut active_ports = HashSet::new();

  for route in routes {
    if !route.enabled || route.listen_port == 0 || route.backend.trim().is_empty() {
      continue;
    }
    if !route_protocol_match(&route, protocol) {
      continue;
    }

    active_ports.insert(route.listen_port);
    let result = if protocol == "tcp" {
      stream_tcp::register(inner, route.listen_port, route.backend.clone()).await
    } else {
      stream_udp::register(inner, route.listen_port, route.backend.clone()).await
    };

    if let Err(err) = result {
      warn!(
          target: "warden::ingress",
          protocol = %protocol,
          listen_port = route.listen_port,
          error = %err,
          "register stream route failed"
      );
    }
  }

  if protocol == "tcp" {
    stream_tcp::unregister_inactive(inner, &active_ports).await;
  } else {
    stream_udp::unregister_inactive(inner, &active_ports).await;
  }
}
