use async_trait::async_trait;
use std::sync::Arc;
use warden_runtime::{RuntimeEngine, RuntimeLaunchResult, RuntimeProvider};
use warden_store::StateStore;
use warden_task::TaskService;
use warden_types::{
  DeployWorkloadRequest, FailoverRequest, MigrateWorkloadRequest, RebalanceRequest,
};

#[derive(Debug, Default)]
struct MockRuntimeProvider;

#[async_trait]
impl RuntimeProvider for MockRuntimeProvider {
  fn name(&self) -> &'static str {
    "docker"
  }

  async fn deploy(
    &self,
    workload_id: &str,
    _req: &DeployWorkloadRequest,
  ) -> anyhow::Result<RuntimeLaunchResult> {
    Ok(RuntimeLaunchResult {
      backend_address: format!("127.0.0.1:{}", 20_000 + workload_id.len()),
    })
  }

  async fn stop(&self, _workload_id: &str) -> anyhow::Result<()> {
    Ok(())
  }
}

#[tokio::test]
async fn migrate_updates_node_and_endpoint_records() -> anyhow::Result<()> {
  let (service, store) = build_service().await;
  let workload = deploy(&service, "migrate-case").await?;

  let moved = service
    .migrate(
      &workload.id,
      &MigrateWorkloadRequest {
        target_node: String::from("node-2"),
        force_stateful: false,
        max_unavailable: 1,
      },
    )
    .await?
    .expect("workload should exist");

  assert_eq!(moved.node_id, "node-2");
  let endpoints = store.list_endpoints_by_workload(&workload.id).await;
  assert_eq!(endpoints.len(), 1);
  assert_eq!(endpoints[0].node_id, "node-2");
  Ok(())
}

#[tokio::test]
async fn failover_moves_running_workloads_from_failed_node() -> anyhow::Result<()> {
  let (service, _store) = build_service().await;
  let one = deploy(&service, "failover-1").await?;
  let two = deploy(&service, "failover-2").await?;
  for workload_id in [&one.id, &two.id] {
    let _ = service
      .migrate(
        workload_id,
        &MigrateWorkloadRequest {
          target_node: String::from("node-failed"),
          force_stateful: false,
          max_unavailable: 1,
        },
      )
      .await?;
  }

  let result = service
    .failover(&FailoverRequest {
      failed_node: String::from("node-failed"),
      target_node: Some(String::from("node-recover")),
      force_stateful: false,
      max_unavailable: 1,
      max_migrations: None,
    })
    .await?;

  assert_eq!(result.moved.len(), 2);
  assert!(result.skipped.is_empty());
  Ok(())
}

#[tokio::test]
async fn rebalance_moves_workload_when_distribution_is_skewed() -> anyhow::Result<()> {
  let (service, _store) = build_service().await;
  let w1 = deploy(&service, "rebalance-1").await?;
  let _w2 = deploy(&service, "rebalance-2").await?;
  let _w3 = deploy(&service, "rebalance-3").await?;
  let _w4 = deploy(&service, "rebalance-4").await?;

  let _ = service
    .migrate(
      &w1.id,
      &MigrateWorkloadRequest {
        target_node: String::from("node-2"),
        force_stateful: false,
        max_unavailable: 1,
      },
    )
    .await?;

  let result = service
    .rebalance(&RebalanceRequest { max_migrations: 2 })
    .await?;
  assert_eq!(result.moved.len(), 1);
  Ok(())
}

async fn build_service() -> (TaskService, StateStore) {
  let store = StateStore::new();
  let runtime = RuntimeEngine::new();
  runtime
    .register_provider(Arc::new(MockRuntimeProvider))
    .await;
  (
    TaskService::with_nodes(
      runtime,
      store.clone(),
      None,
      String::from("node-1"),
      Vec::new(),
    ),
    store,
  )
}

async fn deploy(
  service: &TaskService,
  name: &str,
) -> anyhow::Result<warden_types::WorkloadSummary> {
  service
    .deploy(DeployWorkloadRequest {
      name: name.to_string(),
      runtime: String::from("docker"),
      image: None,
      firecracker_config: None,
      firecracker_kernel_image: None,
      firecracker_rootfs: None,
      host: None,
      path_prefix: None,
      service_port: None,
      ingress_port: None,
      backend: None,
      process_command: None,
      process_args: Vec::new(),
      process_env: std::collections::BTreeMap::new(),
      process_cwd: None,
    })
    .await
}
