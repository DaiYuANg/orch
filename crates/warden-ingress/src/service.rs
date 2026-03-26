use crate::state::IngressService;
use tokio::net::TcpListener;
use tracing::{error, info};
use warden_ingress_http::build_router;
use warden_ingress_resolver::load_snapshot;
use warden_ingress_types::normalize_bind_addr;

impl IngressService {
  pub async fn start(&self) {
    info!(
        target: "warden::ingress",
        http_addr = %self.inner.listen_addr,
        "ingress startup begin"
    );
    self.sync_routes_once().await;
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
    self.inner.stream.shutdown_tcp_listeners().await;
    self.inner.stream.shutdown_udp_listeners().await;
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
    let app = build_router(self.inner.http.clone());

    tokio::spawn(async move {
      let shutdown = async move {
        wait_for_stop(&mut stop_rx).await;
      };
      let serve = axum::serve(listener, app.into_make_service()).with_graceful_shutdown(shutdown);
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
                sync_routes(&inner).await;
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

  async fn sync_routes_once(&self) {
    sync_routes(&self.inner).await;
  }
}

async fn wait_for_stop(stop_rx: &mut tokio::sync::watch::Receiver<bool>) {
  if *stop_rx.borrow() {
    return;
  }
  while stop_rx.changed().await.is_ok() {
    if *stop_rx.borrow() {
      return;
    }
  }
}

async fn sync_routes(inner: &crate::state::IngressInner) {
  let snapshot = load_snapshot(&inner.registry).await;
  inner.http.replace_snapshot(snapshot.clone()).await;
  inner.stream.sync_tcp_routes(&snapshot.tcp_routes).await;
  inner
    .stream
    .sync_udp_routes(&snapshot.udp_routes, inner.options.udp_backend_timeout)
    .await;
}
