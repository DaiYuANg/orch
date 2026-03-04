#![cfg(windows)]

use anyhow::Context;
use tokio::io::copy_bidirectional;
use tokio::net::TcpStream;
use tokio::net::windows::named_pipe::{NamedPipeServer, ServerOptions};
use tokio::sync::broadcast;
use tokio::task::JoinHandle;
use tracing::{info, warn};

pub(crate) fn spawn_npipe_proxy_listener(
  pipe_name: String,
  target_port: u16,
  mut shutdown_rx: broadcast::Receiver<()>,
) -> anyhow::Result<JoinHandle<anyhow::Result<()>>> {
  info!(target: "warden::http", pipe = %pipe_name, "named pipe listener serving");

  Ok(tokio::spawn(async move {
    loop {
      let server = match ServerOptions::new()
        .access_inbound(true)
        .access_outbound(true)
        .create(&pipe_name)
      {
        Ok(server) => server,
        Err(err) => {
          warn!(target: "warden::http", pipe = %pipe_name, error = %err, "create named pipe server failed");
          tokio::time::sleep(std::time::Duration::from_millis(300)).await;
          continue;
        }
      };

      tokio::select! {
          _ = shutdown_rx.recv() => {
              info!(target: "warden::http", pipe = %pipe_name, "named pipe listener stopping");
              break;
          }
          connect_result = server.connect() => {
              if let Err(err) = connect_result {
                  warn!(target: "warden::http", pipe = %pipe_name, error = %err, "named pipe connect failed");
                  continue;
              }
              let backend = format!("127.0.0.1:{target_port}");
              tokio::spawn(async move {
                  if let Err(err) = proxy_pipe_connection(server, backend).await {
                      warn!(target: "warden::http", error = %err, "named pipe proxy connection failed");
                  }
              });
          }
      }
    }
    info!(target: "warden::http", pipe = %pipe_name, "named pipe listener stopped");
    Ok(())
  }))
}

async fn proxy_pipe_connection(
  mut pipe: NamedPipeServer,
  backend_addr: String,
) -> anyhow::Result<()> {
  let mut backend = TcpStream::connect(&backend_addr)
    .await
    .with_context(|| format!("connect backend tcp {}", backend_addr))?;
  let _ = copy_bidirectional(&mut pipe, &mut backend)
    .await
    .context("proxy named pipe data stream")?;
  Ok(())
}
