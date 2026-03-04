#![cfg(unix)]

use anyhow::Context;
use axum::Router;
use std::path::PathBuf;
use tokio::fs;
use tokio::net::UnixListener;
use tokio::sync::broadcast;
use tokio::task::JoinHandle;
use tracing::{error, info};

pub(crate) async fn spawn_uds_listener(
  socket_path: &str,
  app: Router,
  mut shutdown_rx: broadcast::Receiver<()>,
) -> anyhow::Result<JoinHandle<anyhow::Result<()>>> {
  let path = PathBuf::from(socket_path);
  if let Some(parent) = path.parent() {
    fs::create_dir_all(parent)
      .await
      .with_context(|| format!("create uds parent dir: {}", parent.display()))?;
  }
  if path.exists() {
    let _ = fs::remove_file(&path).await;
  }

  let uds_listener = UnixListener::bind(&path)
    .with_context(|| format!("bind uds listener on {}", path.display()))?;
  info!(target: "warden::http", socket = %path.display(), "uds listener serving");

  let cleanup_path = path.clone();
  Ok(tokio::spawn(async move {
    let result = axum::serve(uds_listener, app)
      .with_graceful_shutdown(async move {
        let _ = shutdown_rx.recv().await;
      })
      .await
      .context("uds server stopped unexpectedly");

    if let Err(err) = fs::remove_file(&cleanup_path).await {
      error!(target: "warden::http", error = %err, socket = %cleanup_path.display(), "remove uds file failed");
    }
    result
  }))
}
