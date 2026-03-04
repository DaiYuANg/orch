use crate::TaskService;
use tracing::{info, warn};
use warden_types::TaskLogsResponse;

impl TaskService {
  pub async fn logs(
    &self,
    workload_id: &str,
    tail: usize,
  ) -> anyhow::Result<Option<TaskLogsResponse>> {
    let limit = tail.max(1);
    let workload = match self.store.get_workload(workload_id).await {
      Some(item) => item,
      None => {
        warn!(
          target: "warden::task",
          workload_id = %workload_id,
          "logs requested for missing workload"
        );
        return Ok(None);
      }
    };
    let lines = self
      .runtime
      .logs(workload_id, &workload.runtime, limit)
      .await?;

    let response = TaskLogsResponse {
      workload_id: workload_id.to_string(),
      runtime: workload.runtime,
      tail: limit,
      lines,
    };
    info!(
      target: "warden::task",
      workload_id = %workload_id,
      runtime = %response.runtime,
      tail = limit,
      lines = response.lines.len(),
      "task logs resolved"
    );
    Ok(Some(response))
  }
}
