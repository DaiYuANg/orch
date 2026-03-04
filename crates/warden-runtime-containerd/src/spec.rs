use crate::helper::{self, ContainerdRuntimeConfig};
use prost_types::Any;
use serde_json::json;
use std::fs;
use warden_types::DeployWorkloadRequest;

const SPEC_TYPE_URL: &str = "types.containerd.io/opencontainers/runtime-spec/1/Spec";

pub(crate) enum SpecRoot {
  Snapshot,
  HostPath(String),
}

pub(crate) fn resolve_container_spec(
  cfg: &ContainerdRuntimeConfig,
  root: SpecRoot,
) -> anyhow::Result<Any> {
  if let Some(spec_path) = cfg.spec_path.as_deref() {
    let payload = fs::read(spec_path)
      .map_err(|err| anyhow::anyhow!("read containerd spec file {} failed: {}", spec_path, err))?;
    return Ok(Any {
      type_url: SPEC_TYPE_URL.to_string(),
      value: payload,
    });
  }

  let root_path = match root {
    SpecRoot::Snapshot => String::from("rootfs"),
    SpecRoot::HostPath(path) => path,
  };
  let spec = json!({
    "ociVersion": "1.0.2",
    "process": {
      "terminal": false,
      "args": ["/bin/sh", "-c", "while true; do sleep 3600; done"],
      "cwd": "/",
      "env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
      ],
      "user": { "uid": 0, "gid": 0 }
    },
    "root": { "path": root_path, "readonly": false },
    "hostname": "warden",
    "linux": {
      "namespaces": [
        { "type": "pid" },
        { "type": "network" },
        { "type": "ipc" },
        { "type": "uts" },
        { "type": "mount" }
      ]
    }
  });
  let value = serde_json::to_vec(&spec)?;
  Ok(Any {
    type_url: SPEC_TYPE_URL.to_string(),
    value,
  })
}

pub(crate) fn resolve_host_rootfs(
  cfg: &ContainerdRuntimeConfig,
  req: &DeployWorkloadRequest,
) -> anyhow::Result<String> {
  let req_rootfs = req
    .image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
    .filter(|v| helper::looks_like_filesystem_path(v))
    .map(ToString::to_string);
  req_rootfs.or_else(|| cfg.rootfs.clone()).ok_or_else(|| {
    anyhow::anyhow!(
      "containerd rootfs is required for filesystem deploy: set WARDEN_CONTAINERD_ROOTFS or WARDEN_CONTAINERD_SPEC_PATH"
    )
  })
}
