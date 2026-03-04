use crate::TaskService;
use std::collections::HashMap;
use warden_types::{
  BatchActionResult, FailoverRequest, MigrateWorkloadRequest, RebalanceRequest, WorkloadSummary,
};

impl TaskService {
  pub async fn migrate(
    &self,
    workload_id: &str,
    req: &MigrateWorkloadRequest,
  ) -> anyhow::Result<Option<WorkloadSummary>> {
    let target = req.target_node.trim();
    if target.is_empty() {
      anyhow::bail!("target_node is required");
    }
    validate_availability_budget(req.max_unavailable)?;

    let mut workload = match self.store.get_workload(workload_id).await {
      Some(item) => item,
      None => return Ok(None),
    };
    if workload.node_id == target {
      return Ok(Some(workload));
    }

    workload.node_id = target.to_string();
    self.store.upsert_workload(workload.clone()).await?;
    let _ = self
      .store
      .replace_workload_endpoint_node(workload_id, target)
      .await?;
    Ok(Some(workload))
  }

  pub async fn failover(&self, req: &FailoverRequest) -> anyhow::Result<BatchActionResult> {
    let failed_node = req.failed_node.trim();
    if failed_node.is_empty() {
      anyhow::bail!("failed_node is required");
    }
    validate_availability_budget(req.max_unavailable)?;

    let target_node = pick_target_node(failed_node, req.target_node.as_deref())?;
    let workloads = self.store.list_workloads().await;
    let mut candidates = workloads
      .into_iter()
      .filter(|w| w.node_id == failed_node && w.status.eq_ignore_ascii_case("running"))
      .collect::<Vec<_>>();
    let limit = req.max_migrations.unwrap_or(candidates.len());
    if limit < candidates.len() {
      candidates.truncate(limit);
    }
    apply_migrations(self, &candidates, &target_node).await
  }

  pub async fn rebalance(&self, req: &RebalanceRequest) -> anyhow::Result<BatchActionResult> {
    let mut workloads = self
      .store
      .list_workloads()
      .await
      .into_iter()
      .filter(|w| w.status.eq_ignore_ascii_case("running"))
      .collect::<Vec<_>>();
    if workloads.len() < 2 {
      return Ok(empty_result("no rebalance needed"));
    }

    let limit = req.max_migrations.max(1);
    let mut moved = Vec::new();
    let mut skipped = Vec::new();
    for _ in 0..limit {
      let counts = node_counts(&workloads);
      let Some((source, source_count)) = most_loaded(&counts) else {
        break;
      };
      let Some((target, target_count)) = least_loaded(&counts) else {
        break;
      };
      if source == target || source_count <= target_count + 1 {
        break;
      }

      let Some(idx) = workloads.iter().position(|w| w.node_id == source) else {
        skipped.push(format!("no workload found on source node {source}"));
        break;
      };
      let workload_id = workloads[idx].id.clone();
      let req = MigrateWorkloadRequest {
        target_node: target.clone(),
        force_stateful: false,
        max_unavailable: 1,
      };
      match self.migrate(&workload_id, &req).await {
        Ok(Some(item)) => {
          workloads[idx] = item.clone();
          moved.push(item.id);
        }
        Ok(None) => skipped.push(format!("{workload_id}: not found")),
        Err(err) => skipped.push(format!("{workload_id}: {err}")),
      }
    }

    Ok(BatchActionResult {
      moved,
      skipped,
      message: String::from("rebalance completed"),
    })
  }
}

async fn apply_migrations(
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

fn pick_target_node(failed_node: &str, target_node: Option<&str>) -> anyhow::Result<String> {
  let picked = target_node.map(str::trim).unwrap_or("node-1");
  if picked.is_empty() {
    anyhow::bail!("target_node is required when failover target is empty");
  }
  if picked == failed_node {
    anyhow::bail!("target_node must be different from failed_node");
  }
  Ok(picked.to_string())
}

fn validate_availability_budget(max_unavailable: u32) -> anyhow::Result<()> {
  if max_unavailable == 1 {
    Ok(())
  } else {
    anyhow::bail!("currently only max_unavailable=1 is supported")
  }
}

fn empty_result(message: &str) -> BatchActionResult {
  BatchActionResult {
    moved: Vec::new(),
    skipped: Vec::new(),
    message: message.to_string(),
  }
}

fn node_counts(workloads: &[WorkloadSummary]) -> HashMap<String, usize> {
  workloads.iter().fold(HashMap::new(), |mut acc, item| {
    *acc.entry(item.node_id.clone()).or_insert(0) += 1;
    acc
  })
}

fn most_loaded(counts: &HashMap<String, usize>) -> Option<(String, usize)> {
  counts
    .iter()
    .max_by_key(|(_, count)| **count)
    .map(|(node, count)| (node.clone(), *count))
}

fn least_loaded(counts: &HashMap<String, usize>) -> Option<(String, usize)> {
  counts
    .iter()
    .min_by_key(|(_, count)| **count)
    .map(|(node, count)| (node.clone(), *count))
}
