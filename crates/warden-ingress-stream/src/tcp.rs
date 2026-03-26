use crate::runtime::TcpListenerHandle;
use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use tokio::io::copy_bidirectional;
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::{Mutex, RwLock, oneshot};
use tracing::warn;

pub(crate) async fn register(
  listeners: &Mutex<HashMap<u16, TcpListenerHandle>>,
  listen_port: u16,
  backend: String,
) -> anyhow::Result<()> {
  let mut listeners = listeners.lock().await;
  if let Some(existing) = listeners.get(&listen_port) {
    *existing.backend.write().await = backend;
    return Ok(());
  }

  let listener = TcpListener::bind(format!("0.0.0.0:{listen_port}")).await?;
  let (stop_tx, stop_rx) = oneshot::channel();
  let backend_ref = Arc::new(RwLock::new(backend));
  listeners.insert(
    listen_port,
    TcpListenerHandle {
      backend: backend_ref.clone(),
      stop_tx,
    },
  );
  tokio::spawn(run_listener(listener, backend_ref, stop_rx));
  Ok(())
}

async fn run_listener(
  listener: TcpListener,
  backend: Arc<RwLock<String>>,
  mut stop_rx: oneshot::Receiver<()>,
) {
  loop {
    tokio::select! {
      _ = &mut stop_rx => return,
      accepted = listener.accept() => match accepted {
        Ok((stream, _)) => {
          let backend_ref = backend.clone();
          tokio::spawn(async move {
            handle_connection(stream, backend_ref).await;
          });
        }
        Err(err) => warn!(target: "warden::ingress", error = %err, "tcp accept error"),
      },
    }
  }
}

async fn handle_connection(mut client: TcpStream, backend: Arc<RwLock<String>>) {
  let backend_addr = backend.read().await.clone();
  let mut upstream = match TcpStream::connect(&backend_addr).await {
    Ok(item) => item,
    Err(err) => {
      warn!(
        target: "warden::ingress",
        backend = %backend_addr,
        error = %err,
        "tcp dial backend error"
      );
      return;
    }
  };
  if let Err(err) = copy_bidirectional(&mut client, &mut upstream).await {
    warn!(target: "warden::ingress", error = %err, "tcp proxy copy error");
  }
}

pub(crate) async fn unregister_inactive(
  listeners: &Mutex<HashMap<u16, TcpListenerHandle>>,
  active_ports: &HashSet<u16>,
) {
  let mut listeners = listeners.lock().await;
  let stale = listeners
    .keys()
    .filter(|port| !active_ports.contains(port))
    .copied()
    .collect::<Vec<_>>();
  for port in stale {
    if let Some(handle) = listeners.remove(&port) {
      let _ = handle.stop_tx.send(());
    }
  }
}
