use crate::tcp;
use crate::udp;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::{Mutex, RwLock, oneshot};
use tracing::warn;
use warden_ingress_types::StreamRoute;

pub struct StreamIngressRuntime {
  pub(crate) tcp_listeners: Mutex<HashMap<u16, TcpListenerHandle>>,
  pub(crate) udp_listeners: Mutex<HashMap<u16, UdpListenerHandle>>,
}

pub(crate) struct TcpListenerHandle {
  pub(crate) backend: Arc<RwLock<String>>,
  pub(crate) stop_tx: oneshot::Sender<()>,
}

pub(crate) struct UdpListenerHandle {
  pub(crate) backend: Arc<RwLock<String>>,
  pub(crate) stop_tx: oneshot::Sender<()>,
}

impl StreamIngressRuntime {
  pub fn new() -> Self {
    Self {
      tcp_listeners: Mutex::new(HashMap::new()),
      udp_listeners: Mutex::new(HashMap::new()),
    }
  }

  pub async fn sync_tcp_routes(&self, routes: &[StreamRoute]) {
    let active_ports = routes.iter().map(|route| route.listen_port).collect();

    for route in routes {
      if let Err(err) = tcp::register(
        &self.tcp_listeners,
        route.listen_port,
        route.backend.clone(),
      )
      .await
      {
        warn!(
          target: "warden::ingress",
          protocol = "tcp",
          listen_port = route.listen_port,
          error = %err,
          "register stream route failed"
        );
      }
    }

    tcp::unregister_inactive(&self.tcp_listeners, &active_ports).await;
  }

  pub async fn sync_udp_routes(&self, routes: &[StreamRoute], udp_backend_timeout: Duration) {
    let active_ports = routes.iter().map(|route| route.listen_port).collect();

    for route in routes {
      if let Err(err) = udp::register(
        &self.udp_listeners,
        route.listen_port,
        route.backend.clone(),
        udp_backend_timeout,
      )
      .await
      {
        warn!(
          target: "warden::ingress",
          protocol = "udp",
          listen_port = route.listen_port,
          error = %err,
          "register stream route failed"
        );
      }
    }

    udp::unregister_inactive(&self.udp_listeners, &active_ports).await;
  }

  pub async fn shutdown_tcp_listeners(&self) {
    let mut listeners = self.tcp_listeners.lock().await;
    for (_, handle) in listeners.drain() {
      let _ = handle.stop_tx.send(());
    }
  }

  pub async fn shutdown_udp_listeners(&self) {
    let mut listeners = self.udp_listeners.lock().await;
    for (_, handle) in listeners.drain() {
      let _ = handle.stop_tx.send(());
    }
  }
}
