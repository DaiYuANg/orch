use crate::TaskService;
use warden_types::{BatchActionResult, MigrateWorkloadRequest, WorkloadSummary};

pub(crate) async fn apply_migrations(
  service: &TaskService,
  candidates: &[WorkloadSummary],
  target_node: &str,
) -> anyhow::Result<BatchActionResult> {
  let mut moved = Vec::new();
  let mut skipped = Vec::new();
  for item in candidates {
    let req = MigrateWorkloadRequest {
      target_node: target_node.to_string(),
      force_stateful: false,
      max_unavailable: 1,
    };
    match service.migrate(&item.id, &req).await {
      Ok(Some(value)) => moved.push(value.id),
      Ok(None) => skipped.push(format!("{}: not found", item.id)),
      Err(err) => skipped.push(format!("{}: {err}", item.id)),
    }
  }
  Ok(BatchActionResult {
    moved,
    skipped,
    message: String::from("failover completed"),
  })
}

pub(crate) fn validate_availability_budget(max_unavailable: u32) -> anyhow::Result<()> {
  if max_unavailable == 1 {
    Ok(())
  } else {
    anyhow::bail!("currently only max_unavailable=1 is supported")
  }
}

pub(crate) fn empty_result(message: &str) -> BatchActionResult {
  BatchActionResult {
    moved: Vec::new(),
    skipped: Vec::new(),
    message: message.to_string(),
  }
}

pub(crate) fn most_loaded(
  counts: &std::collections::HashMap<String, usize>,
) -> Option<(String, usize)> {
  counts
    .iter()
    .max_by_key(|(_, count)| **count)
    .map(|(node, count)| (node.clone(), *count))
}

pub(crate) fn least_loaded(
  counts: &std::collections::HashMap<String, usize>,
) -> Option<(String, usize)> {
  counts
    .iter()
    .min_by_key(|(_, count)| **count)
    .map(|(node, count)| (node.clone(), *count))
}
