use crate::runtime::UdpListenerHandle;
use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use std::time::Duration;
use tokio::net::UdpSocket;
use tokio::sync::{Mutex, RwLock, oneshot};
use tokio::time::timeout;
use tracing::warn;

pub(crate) async fn register(
  listeners: &Mutex<HashMap<u16, UdpListenerHandle>>,
  listen_port: u16,
  backend: String,
  udp_backend_timeout: Duration,
) -> anyhow::Result<()> {
  let mut listeners = listeners.lock().await;
  if let Some(existing) = listeners.get(&listen_port) {
    *existing.backend.write().await = backend;
    return Ok(());
  }

  let socket = Arc::new(UdpSocket::bind(format!("0.0.0.0:{listen_port}")).await?);
  let (stop_tx, stop_rx) = oneshot::channel();
  let backend_ref = Arc::new(RwLock::new(backend));
  listeners.insert(
    listen_port,
    UdpListenerHandle {
      backend: backend_ref.clone(),
      stop_tx,
    },
  );
  tokio::spawn(run_listener(
    socket,
    backend_ref,
    stop_rx,
    udp_backend_timeout,
  ));
  Ok(())
}

async fn run_listener(
  socket: Arc<UdpSocket>,
  backend: Arc<RwLock<String>>,
  mut stop_rx: oneshot::Receiver<()>,
  udp_backend_timeout: Duration,
) {
  let mut buf = vec![0u8; 64 * 1024];
  loop {
    tokio::select! {
      _ = &mut stop_rx => return,
      packet = socket.recv_from(&mut buf) => match packet {
        Ok((n, client_addr)) => {
          let payload = buf[..n].to_vec();
          let backend_ref = backend.clone();
          let listener_socket = socket.clone();
          tokio::spawn(async move {
            handle_packet(
              listener_socket,
              backend_ref,
              client_addr,
              payload,
              udp_backend_timeout,
            )
            .await;
          });
        }
        Err(err) => warn!(target: "warden::ingress", error = %err, "udp read error"),
      },
    }
  }
}

async fn handle_packet(
  listener_socket: Arc<UdpSocket>,
  backend: Arc<RwLock<String>>,
  client_addr: std::net::SocketAddr,
  payload: Vec<u8>,
  udp_backend_timeout: Duration,
) {
  let backend_addr = backend.read().await.clone();
  let upstream = match UdpSocket::bind("0.0.0.0:0").await {
    Ok(item) => item,
    Err(err) => {
      warn!(target: "warden::ingress", error = %err, "udp local bind error");
      return;
    }
  };

  if let Err(err) = upstream.send_to(&payload, &backend_addr).await {
    warn!(target: "warden::ingress", backend = %backend_addr, error = %err, "udp write to backend error");
    return;
  }

  let mut resp = vec![0u8; 64 * 1024];
  let read = timeout(udp_backend_timeout, upstream.recv_from(&mut resp)).await;
  let n = match read {
    Ok(Ok((n, _))) => n,
    Ok(Err(err)) => {
      warn!(target: "warden::ingress", backend = %backend_addr, error = %err, "udp read from backend error");
      return;
    }
    Err(_) => {
      warn!(target: "warden::ingress", backend = %backend_addr, "udp read from backend timeout");
      return;
    }
  };

  if let Err(err) = listener_socket.send_to(&resp[..n], client_addr).await {
    warn!(target: "warden::ingress", error = %err, "udp write to client error");
  }
}

pub(crate) async fn unregister_inactive(
  listeners: &Mutex<HashMap<u16, UdpListenerHandle>>,
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
