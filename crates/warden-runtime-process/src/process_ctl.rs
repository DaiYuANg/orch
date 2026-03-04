use anyhow::Context;
use tokio::process::Child;
use tokio::time::{Duration, timeout};
use tracing::warn;

pub(crate) async fn stop_child(
  workload_id: &str,
  mut child: Child,
  stop_timeout: Duration,
) -> anyhow::Result<()> {
  if child.try_wait().context("read process status")?.is_some() {
    return Ok(());
  }

  child.start_kill().context("kill managed process")?;
  match timeout(stop_timeout, child.wait()).await {
    Ok(wait_result) => {
      let _ = wait_result.context("wait for managed process to exit")?;
      Ok(())
    }
    Err(_) => {
      warn!(
        target: "warden::runtime::process",
        workload_id = %workload_id,
        timeout_ms = stop_timeout.as_millis(),
        "process did not exit in time; forcing kill"
      );
      child.start_kill().ok();
      let _ = child.wait().await;
      Ok(())
    }
  }
}
