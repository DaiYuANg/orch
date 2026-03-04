use crate::http_proxy::proxy_http_handler;
use crate::state::IngressService;
use crate::stream::sync_stream_routes;
use crate::util::{normalize_bind_addr, wait_for_stop};
use axum::Router;
use axum::routing::any;
use tokio::net::TcpListener;
use tracing::{error, info};

impl IngressService {
  pub async fn start(&self) {
    info!(
        target: "warden::ingress",
        http_addr = %self.inner.listen_addr,
        "ingress startup begin"
    );
    self.start_http_listener().await;
    self.start_stream_sync_loop();
    info!(
        target: "warden::ingress",
        http_addr = %self.inner.listen_addr,
        "ingress startup complete"
    );
  }

  pub async fn stop(&self) {
    info!(target: "warden::ingress", "ingress shutdown begin");
    let _ = self.inner.stop_tx.send(true);
    self.shutdown_tcp_listeners().await;
    self.shutdown_udp_listeners().await;
    self.inner.http_cache.invalidate_all();
    info!(target: "warden::ingress", "ingress shutdown complete");
  }

  async fn start_http_listener(&self) {
    let bind_addr = normalize_bind_addr(&self.inner.listen_addr);
    let listener = match TcpListener::bind(&bind_addr).await {
      Ok(item) => item,
      Err(err) => {
        error!(
            target: "warden::ingress",
            addr = %bind_addr,
            error = %err,
            "ingress http listen failed"
        );
        return;
      }
    };

    let mut stop_rx = self.inner.stop_tx.subscribe();
    let app = Router::new()
      .fallback(any(proxy_http_handler))
      .with_state(self.inner.clone());

    tokio::spawn(async move {
      let serve =
        axum::serve(listener, app.into_make_service()).with_graceful_shutdown(async move {
          wait_for_stop(&mut stop_rx).await;
        });
      if let Err(err) = serve.await {
        error!(
            target: "warden::ingress",
            error = %err,
            "ingress http listener stopped with error"
        );
        return;
      }
      info!(target: "warden::ingress", "ingress http listener stopped");
    });
  }

  fn start_stream_sync_loop(&self) {
    let inner = self.inner.clone();
    let mut stop_rx = inner.stop_tx.subscribe();
    tokio::spawn(async move {
      let mut ticker = tokio::time::interval(inner.options.stream_sync_interval);
      loop {
        tokio::select! {
            _ = ticker.tick() => {
                sync_stream_routes(&inner, "tcp").await;
                sync_stream_routes(&inner, "udp").await;
            }
            _ = stop_rx.changed() => {
                if *stop_rx.borrow() {
                    return;
                }
            }
        }
      }
    });
  }

  async fn shutdown_tcp_listeners(&self) {
    let mut listeners = self.inner.tcp_listeners.lock().await;
    for (_, handle) in listeners.drain() {
      let _ = handle.stop_tx.send(());
    }
  }

  async fn shutdown_udp_listeners(&self) {
    let mut listeners = self.inner.udp_listeners.lock().await;
    for (_, handle) in listeners.drain() {
      let _ = handle.stop_tx.send(());
    }
  }
}
