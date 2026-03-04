use crate::helper::{self, ContainerdRuntimeConfig};
use crate::oci;
use crate::spec::{self, SpecRoot};
use containerd_client::services::v1::{
  Container, CreateContainerRequest, CreateTaskRequest, StartRequest, container::Runtime,
};
use containerd_client::tonic::Request;
use containerd_client::types::Mount;
use containerd_client::with_namespace;
use std::collections::HashMap;
use warden_types::DeployWorkloadRequest;

struct DeployPlan {
  image: String,
  snapshot_key: Option<String>,
  task_rootfs: Vec<Mount>,
  spec: prost_types::Any,
}

pub(crate) async fn deploy_workload(
  cfg: &ContainerdRuntimeConfig,
  workload_id: &str,
  req: &DeployWorkloadRequest,
) -> anyhow::Result<String> {
  let container_name = helper::container_name(workload_id);
  cleanup_container_and_snapshot(cfg, &container_name).await?;

  let plan = plan_deploy(cfg, req, &container_name).await?;
  let mut labels = HashMap::new();
  labels.insert(
    String::from("warden.workload_id"),
    workload_id.trim().to_string(),
  );
  labels.insert(String::from("warden.managed"), String::from("true"));

  let client = helper::connect(cfg).await?;
  let ns = cfg.namespace.as_str();
  let mut containers = client.containers();
  let create_container = CreateContainerRequest {
    container: Some(Container {
      id: container_name.clone(),
      labels,
      image: plan.image.clone(),
      runtime: Some(Runtime {
        name: cfg.runtime_name.clone(),
        options: None,
      }),
      snapshotter: plan
        .snapshot_key
        .as_ref()
        .map(|_| cfg.snapshotter.clone())
        .unwrap_or_default(),
      snapshot_key: plan.snapshot_key.unwrap_or_default(),
      spec: Some(plan.spec),
      ..Default::default()
    }),
  };
  let create_container = with_namespace!(create_container, ns);
  if let Err(err) = containers.create(create_container).await {
    return Err(anyhow::anyhow!(
      "create container {} failed: {}",
      container_name,
      err
    ));
  }

  let mut tasks = client.tasks();
  let create_task = CreateTaskRequest {
    container_id: container_name.clone(),
    rootfs: plan.task_rootfs,
    ..Default::default()
  };
  let create_task = with_namespace!(create_task, ns);
  if let Err(err) = tasks.create(create_task).await {
    let _ = cleanup_container_and_snapshot(cfg, &container_name).await;
    return Err(anyhow::anyhow!(
      "create task {} failed: {}",
      container_name,
      err
    ));
  }

  let start = StartRequest {
    container_id: container_name.clone(),
    ..Default::default()
  };
  let start = with_namespace!(start, ns);
  if let Err(err) = tasks.start(start).await {
    let _ = cleanup_container_and_snapshot(cfg, &container_name).await;
    return Err(anyhow::anyhow!(
      "start task {} failed: {}",
      container_name,
      err
    ));
  }

  Ok(plan.image)
}

pub(crate) async fn cleanup_container_and_snapshot(
  cfg: &ContainerdRuntimeConfig,
  container_name: &str,
) -> anyhow::Result<()> {
  helper::remove_existing(cfg, container_name).await?;
  let snapshot_key = oci::snapshot_key(container_name);
  oci::remove_snapshot(cfg, &snapshot_key).await
}

async fn plan_deploy(
  cfg: &ContainerdRuntimeConfig,
  req: &DeployWorkloadRequest,
  container_name: &str,
) -> anyhow::Result<DeployPlan> {
  if is_oci_image_ref(req) {
    let image = helper::resolve_image(req);
    let snapshot = oci::prepare_snapshot_rootfs(cfg, container_name, &image).await?;
    let spec = spec::resolve_container_spec(cfg, SpecRoot::Snapshot)?;
    return Ok(DeployPlan {
      image,
      snapshot_key: Some(snapshot.snapshot_key),
      task_rootfs: snapshot.mounts,
      spec,
    });
  }

  let image = resolve_container_image(req);
  let rootfs = spec::resolve_host_rootfs(cfg, req)?;
  let spec = spec::resolve_container_spec(cfg, SpecRoot::HostPath(rootfs))?;
  Ok(DeployPlan {
    image,
    snapshot_key: None,
    task_rootfs: Vec::new(),
    spec,
  })
}

fn resolve_container_image(req: &DeployWorkloadRequest) -> String {
  let raw = helper::resolve_image(req);
  if helper::looks_like_filesystem_path(&raw) {
    String::from("local/rootfs")
  } else {
    raw
  }
}

fn is_oci_image_ref(req: &DeployWorkloadRequest) -> bool {
  req
    .image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .map(|v| !helper::looks_like_filesystem_path(v))
    .unwrap_or(true)
}
