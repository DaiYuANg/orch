use crate::helper::{self, ContainerdRuntimeConfig};
use crate::oci_parent::collect_snapshot_parent_candidates;
use crate::oci_transfer;
use containerd_client::services::v1::GetImageRequest;
use containerd_client::services::v1::snapshots::{
  MountsRequest, PrepareSnapshotRequest, RemoveSnapshotRequest,
};
use containerd_client::tonic;
use containerd_client::tonic::Request;
use containerd_client::types::Mount;
use containerd_client::with_namespace;
use std::collections::HashMap;

#[derive(Debug)]
pub(crate) struct SnapshotRootfs {
  pub snapshot_key: String,
  pub mounts: Vec<Mount>,
}

pub(crate) fn snapshot_key(container_name: &str) -> String {
  format!("{container_name}-snapshot")
}

pub(crate) async fn remove_snapshot(
  cfg: &ContainerdRuntimeConfig,
  key: &str,
) -> anyhow::Result<()> {
  let client = helper::connect(cfg).await?;
  let ns = cfg.namespace.as_str();
  let mut snapshots = client.snapshots();
  let request = RemoveSnapshotRequest {
    snapshotter: cfg.snapshotter.clone(),
    key: key.to_string(),
  };
  let request = with_namespace!(request, ns);
  match snapshots.remove(request).await {
    Ok(_) => Ok(()),
    Err(err) if err.code() == tonic::Code::NotFound => Ok(()),
    Err(err) => Err(anyhow::anyhow!(
      "remove snapshot {} on {} failed: {}",
      key,
      cfg.snapshotter,
      err
    )),
  }
}

pub(crate) async fn prepare_snapshot_rootfs(
  cfg: &ContainerdRuntimeConfig,
  container_name: &str,
  image: &str,
) -> anyhow::Result<SnapshotRootfs> {
  oci_transfer::ensure_oci_image_ready(cfg, image).await?;
  let snapshot_key = snapshot_key(container_name);
  let _ = remove_snapshot(cfg, &snapshot_key).await;

  let parents = snapshot_parent_candidates(cfg, image).await?;
  let mut last_error = String::new();
  for parent in parents {
    match prepare_snapshot_once(cfg, &snapshot_key, &parent).await {
      Ok(mounts) => {
        return Ok(SnapshotRootfs {
          snapshot_key,
          mounts,
        });
      }
      Err(err) => {
        last_error = format!("parent={parent}, error={err}");
      }
    }
  }

  Err(anyhow::anyhow!(
    "prepare snapshot rootfs failed for image {}: {}",
    image,
    last_error
  ))
}

async fn prepare_snapshot_once(
  cfg: &ContainerdRuntimeConfig,
  key: &str,
  parent: &str,
) -> anyhow::Result<Vec<Mount>> {
  let client = helper::connect(cfg).await?;
  let ns = cfg.namespace.as_str();
  let mut snapshots = client.snapshots();
  let request = PrepareSnapshotRequest {
    snapshotter: cfg.snapshotter.clone(),
    key: key.to_string(),
    parent: parent.to_string(),
    labels: HashMap::new(),
  };
  let request = with_namespace!(request, ns);
  match snapshots.prepare(request).await {
    Ok(resp) => Ok(resp.into_inner().mounts),
    Err(err) if err.code() == tonic::Code::AlreadyExists => {
      let mounts = MountsRequest {
        snapshotter: cfg.snapshotter.clone(),
        key: key.to_string(),
      };
      let mounts = with_namespace!(mounts, ns);
      let response = snapshots.mounts(mounts).await.map_err(|inner| {
        anyhow::anyhow!(
          "resolve mounts for existing snapshot {} failed: {}",
          key,
          inner
        )
      })?;
      Ok(response.into_inner().mounts)
    }
    Err(err) => Err(anyhow::anyhow!(
      "prepare snapshot with parent {} failed: {}",
      parent,
      err
    )),
  }
}

async fn snapshot_parent_candidates(
  cfg: &ContainerdRuntimeConfig,
  image: &str,
) -> anyhow::Result<Vec<String>> {
  let client = helper::connect(cfg).await?;
  let ns = cfg.namespace.as_str();

  let mut images = client.images();
  let request = GetImageRequest {
    name: image.to_string(),
  };
  let request = with_namespace!(request, ns);
  let image_meta = images
    .get(request)
    .await
    .ok()
    .and_then(|resp| resp.into_inner().image);
  Ok(collect_snapshot_parent_candidates(
    image_meta.as_ref(),
    &cfg.snapshotter,
    image,
  ))
}
