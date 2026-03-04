use anyhow::Context;
use axum::Router;
use std::net::SocketAddr;
use tokio::net::TcpListener;
use tokio::sync::broadcast;
use tokio::task::JoinHandle;
use tracing::{info, warn};

#[cfg(unix)]
use tracing::error;

pub async fn run(cfg: &warden_config::Config, app: Router) -> anyhow::Result<()> {
    let tcp_addr = SocketAddr::from(([0, 0, 0, 0], cfg.http.port));
    let tcp_listener = TcpListener::bind(tcp_addr)
        .await
        .with_context(|| format!("bind tcp listener on {}", tcp_addr))?;

    info!(target: "warden::http", %tcp_addr, "http listener serving");

    let (shutdown_tx, _) = broadcast::channel::<()>(4);
    let mut handles: Vec<JoinHandle<anyhow::Result<()>>> = Vec::new();

    handles.push(spawn_tcp_listener(
        tcp_listener,
        app.clone(),
        shutdown_tx.subscribe(),
    ));

    #[cfg(unix)]
    {
        let uds = cfg.http.unix_socket.trim();
        if !uds.is_empty() {
            handles.push(spawn_uds_listener(uds, app.clone(), shutdown_tx.subscribe()).await?);
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
            handles.push(spawn_npipe_proxy_listener(
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

    wait_for_shutdown_signal().await;
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

fn spawn_tcp_listener(
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

#[cfg(unix)]
async fn spawn_uds_listener(
    socket_path: &str,
    app: Router,
    mut shutdown_rx: broadcast::Receiver<()>,
) -> anyhow::Result<JoinHandle<anyhow::Result<()>>> {
    use std::path::PathBuf;
    use tokio::fs;
    use tokio::net::UnixListener;

    let path = PathBuf::from(socket_path);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .await
            .with_context(|| format!("create uds parent dir: {}", parent.display()))?;
    }

    if path.exists() {
        let _ = fs::remove_file(path).await;
    }

    let uds_listener = UnixListener::bind(path)
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

async fn wait_for_shutdown_signal() {
    if let Err(err) = tokio::signal::ctrl_c().await {
        warn!(target: "warden::http", error = %err, "ctrl-c signal handler failed");
    }
}

#[cfg(windows)]
fn spawn_npipe_proxy_listener(
    pipe_name: String,
    target_port: u16,
    mut shutdown_rx: broadcast::Receiver<()>,
) -> anyhow::Result<JoinHandle<anyhow::Result<()>>> {
    use tokio::net::windows::named_pipe::ServerOptions;

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

#[cfg(windows)]
async fn proxy_pipe_connection(
    mut pipe: tokio::net::windows::named_pipe::NamedPipeServer,
    backend_addr: String,
) -> anyhow::Result<()> {
    use tokio::io::copy_bidirectional;
    use tokio::net::TcpStream;

    let mut backend = TcpStream::connect(&backend_addr)
        .await
        .with_context(|| format!("connect backend tcp {}", backend_addr))?;
    let _ = copy_bidirectional(&mut pipe, &mut backend)
        .await
        .context("proxy named pipe data stream")?;
    Ok(())
}
