use containerd_client::services::v1::Image;

pub(crate) fn collect_snapshot_parent_candidates(
  image_meta: Option<&Image>,
  snapshotter: &str,
  image: &str,
) -> Vec<String> {
  let mut candidates = Vec::new();
  if let Some(meta) = image_meta {
    for key in snapshot_label_keys(snapshotter) {
      if let Some(value) = meta
        .labels
        .get(&key)
        .map(|v| v.trim())
        .filter(|v| !v.is_empty())
      {
        candidates.push(value.to_string());
      }
    }
    for (key, value) in &meta.labels {
      if key.starts_with("containerd.io/gc.ref.snapshot.") {
        let parent = value.trim();
        if !parent.is_empty() {
          candidates.push(parent.to_string());
        }
      }
    }
    if let Some(digest) = meta
      .target
      .as_ref()
      .map(|target| target.digest.trim())
      .filter(|digest| !digest.is_empty())
    {
      candidates.push(digest.to_string());
    }
  }

  candidates.push(image.to_string());
  candidates.dedup();
  candidates
}

fn snapshot_label_keys(snapshotter: &str) -> Vec<String> {
  let raw = snapshotter.trim();
  let mut keys = Vec::new();
  if !raw.is_empty() {
    keys.push(format!("containerd.io/gc.ref.snapshot.{raw}"));
    if let Some(leaf) = raw.rsplit('.').next().filter(|leaf| !leaf.is_empty()) {
      keys.push(format!("containerd.io/gc.ref.snapshot.{leaf}"));
    }
    if let Some(suffix) = raw.rsplit('/').next().filter(|suffix| !suffix.is_empty()) {
      keys.push(format!("containerd.io/gc.ref.snapshot.{suffix}"));
    }
  }
  keys.dedup();
  keys
}

#[cfg(test)]
mod tests {
  use super::*;
  use containerd_client::types::Descriptor;
  use std::collections::HashMap;

  #[test]
  fn collects_snapshot_parents_with_fallbacks() {
    let mut labels = HashMap::new();
    labels.insert(
      String::from("containerd.io/gc.ref.snapshot.overlayfs"),
      String::from("sha256:parent-overlay"),
    );
    labels.insert(
      String::from("containerd.io/gc.ref.snapshot.stargz"),
      String::from("sha256:parent-stargz"),
    );
    let image_meta = Image {
      name: String::from("docker.io/library/nginx:stable"),
      labels,
      target: Some(Descriptor {
        digest: String::from("sha256:target"),
        ..Default::default()
      }),
      ..Default::default()
    };
    let candidates = collect_snapshot_parent_candidates(
      Some(&image_meta),
      "overlayfs",
      "docker.io/library/nginx:stable",
    );

    assert_eq!(candidates[0], "sha256:parent-overlay");
    assert!(candidates.iter().any(|item| item == "sha256:parent-stargz"));
    assert!(candidates.iter().any(|item| item == "sha256:target"));
    assert_eq!(
      candidates.last().map(String::as_str),
      Some("docker.io/library/nginx:stable")
    );
  }

  #[test]
  fn derives_snapshot_label_keys_from_snapshotter_name() {
    let keys = snapshot_label_keys("io.containerd.snapshotter.v1.overlayfs");
    assert!(
      keys
        .iter()
        .any(|key| key == "containerd.io/gc.ref.snapshot.io.containerd.snapshotter.v1.overlayfs")
    );
    assert!(
      keys
        .iter()
        .any(|key| key == "containerd.io/gc.ref.snapshot.overlayfs")
    );
  }
}
