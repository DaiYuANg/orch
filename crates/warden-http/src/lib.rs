mod signal;
mod tcp;
#[cfg(unix)]
mod unix;
#[cfg(windows)]
mod windows;

use anyhow::Context;
use axum::Router;
use axum::http::{HeaderValue, Method};
use std::net::SocketAddr;
use tokio::net::TcpListener;
use tokio::sync::broadcast;
use tokio::task::JoinHandle;
use tower_http::cors::{Any, CorsLayer};
use tracing::{info, warn};

pub async fn run(cfg: &warden_config::Config, app: Router) -> anyhow::Result<()> {
  let app = app.layer(build_cors_layer());
  let tcp_addr = SocketAddr::from(([0, 0, 0, 0], cfg.http.port));
  let tcp_listener = TcpListener::bind(tcp_addr)
    .await
    .with_context(|| format!("bind tcp listener on {}", tcp_addr))?;
  info!(target: "warden::http", %tcp_addr, "http listener serving");

  let (shutdown_tx, _) = broadcast::channel::<()>(4);
  let mut handles: Vec<JoinHandle<anyhow::Result<()>>> = vec![tcp::spawn_tcp_listener(
    tcp_listener,
    app.clone(),
    shutdown_tx.subscribe(),
  )];

  #[cfg(unix)]
  {
    let uds = cfg.http.unix_socket.trim();
    if !uds.is_empty() {
      handles.push(unix::spawn_uds_listener(uds, app.clone(), shutdown_tx.subscribe()).await?);
    } else {
      info!(target: "warden::http", "uds listener disabled");
    }
  }

  #[cfg(not(unix))]
  {
    if !cfg.http.unix_socket.trim().is_empty() {
      warn!(target: "warden::http", socket = %cfg.http.unix_socket, "uds is not supported on this build target");
    }
  }

  #[cfg(windows)]
  {
    let named_pipe = cfg.http.named_pipe.trim();
    if !named_pipe.is_empty() {
      handles.push(windows::spawn_npipe_proxy_listener(
        named_pipe.to_string(),
        cfg.http.port,
        shutdown_tx.subscribe(),
      )?);
    } else {
      info!(target: "warden::http", "named pipe listener disabled");
    }
  }

  #[cfg(not(windows))]
  {
    if !cfg.http.named_pipe.trim().is_empty() {
      warn!(target: "warden::http", pipe = %cfg.http.named_pipe, "named pipe is configured but unsupported on this build target");
    }
  }

  signal::wait_for_shutdown_signal().await;
  info!(target: "warden::http", "shutdown signal received");
  let _ = shutdown_tx.send(());

  for handle in handles {
    match handle.await {
      Ok(Ok(())) => {}
      Ok(Err(err)) => return Err(err),
      Err(err) => return Err(anyhow::anyhow!("listener task join failed: {err}")),
    }
  }

  info!(target: "warden::http", "all listeners stopped");
  Ok(())
}

fn build_cors_layer() -> CorsLayer {
  CorsLayer::new()
    .allow_origin([
      HeaderValue::from_static("http://127.0.0.1:5173"),
      HeaderValue::from_static("http://localhost:5173"),
    ])
    .allow_methods([
      Method::GET,
      Method::POST,
      Method::PUT,
      Method::PATCH,
      Method::DELETE,
      Method::OPTIONS,
    ])
    .allow_headers(Any)
}
