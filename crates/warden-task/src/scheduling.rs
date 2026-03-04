use crate::TaskService;
use crate::scheduling_helper::{
  apply_migrations, empty_result, least_loaded, most_loaded, validate_availability_budget,
};
use tracing::{info, warn};
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
      None => {
        warn!(
          target: "warden::task",
          workload_id = %workload_id,
          "migrate requested for missing workload"
        );
        return Ok(None);
      }
    };
    if workload.node_id == target {
      info!(
        target: "warden::task",
        workload_id = %workload_id,
        node_id = %target,
        "migrate skipped because workload already on target node"
      );
      return Ok(Some(workload));
    }

    let from_node = workload.node_id.clone();
    info!(
      target: "warden::task",
      workload_id = %workload_id,
      from_node = %from_node,
      to_node = %target,
      "migrate started"
    );
    workload.node_id = target.to_string();
    let moved = self
      .apply_write("task.migrate", || async {
        self.store.upsert_workload(workload.clone()).await?;
        let _ = self
          .store
          .replace_workload_endpoint_node(workload_id, target)
          .await?;
        Ok(Some(workload))
      })
      .await?;
    info!(
      target: "warden::task",
      workload_id = %workload_id,
      from_node = %from_node,
      to_node = %target,
      "migrate completed"
    );
    Ok(moved)
  }

  pub async fn failover(&self, req: &FailoverRequest) -> anyhow::Result<BatchActionResult> {
    let failed_node = req.failed_node.trim();
    if failed_node.is_empty() {
      anyhow::bail!("failed_node is required");
    }
    validate_availability_budget(req.max_unavailable)?;

    let target_node = match req.target_node.as_deref().map(str::trim) {
      Some(node) if node.is_empty() => {
        anyhow::bail!("target_node must not be empty when provided")
      }
      Some(node) if node == failed_node => {
        anyhow::bail!("target_node must be different from failed_node")
      }
      Some(node) => node.to_string(),
      None => self
        .pick_failover_node(failed_node)
        .await
        .ok_or_else(|| anyhow::anyhow!("no available node for failover"))?,
    };
    let workloads = self.store.list_workloads().await;
    let mut candidates = workloads
      .into_iter()
      .filter(|w| w.node_id == failed_node && w.status.eq_ignore_ascii_case("running"))
      .collect::<Vec<_>>();
    let limit = req.max_migrations.unwrap_or(candidates.len());
    if limit < candidates.len() {
      candidates.truncate(limit);
    }
    info!(
      target: "warden::task",
      failed_node = %failed_node,
      target_node = %target_node,
      candidates = candidates.len(),
      "failover started"
    );
    let result = apply_migrations(self, &candidates, &target_node).await?;
    info!(
      target: "warden::task",
      failed_node = %failed_node,
      target_node = %target_node,
      moved = result.moved.len(),
      skipped = result.skipped.len(),
      "failover completed"
    );
    Ok(result)
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
    info!(
      target: "warden::task",
      max_migrations = limit,
      workloads = workloads.len(),
      "rebalance started"
    );
    let mut moved = Vec::new();
    let mut skipped = Vec::new();
    for _ in 0..limit {
      let counts = self.node_loads().await;
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

    let result = BatchActionResult {
      moved,
      skipped,
      message: String::from("rebalance completed"),
    };
    info!(
      target: "warden::task",
      moved = result.moved.len(),
      skipped = result.skipped.len(),
      "rebalance completed"
    );
    Ok(result)
  }
}
