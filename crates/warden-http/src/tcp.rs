use anyhow::Context;
use axum::Router;
use tokio::net::TcpListener;
use tokio::sync::broadcast;
use tokio::task::JoinHandle;

pub(crate) fn spawn_tcp_listener(
  listener: TcpListener,
  app: Router,
  mut shutdown_rx: broadcast::Receiver<()>,
) -> JoinHandle<anyhow::Result<()>> {
  tokio::spawn(async move {
    axum::serve(listener, app)
      .with_graceful_shutdown(async move {
        let _ = shutdown_rx.recv().await;
      })
      .await
      .context("tcp server stopped unexpectedly")
  })
}
