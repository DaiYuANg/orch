use tracing::warn;

pub(crate) async fn wait_for_shutdown_signal() {
  if let Err(err) = tokio::signal::ctrl_c().await {
    warn!(target: "warden::http", error = %err, "ctrl-c signal handler failed");
  }
}
