use crate::helper::{self, ContainerdRuntimeConfig};
use containerd_client::services::v1::{GetImageRequest, TransferRequest};
use containerd_client::tonic::Request;
use containerd_client::with_namespace;
use prost::Message;
use prost_types::Any;
use std::collections::HashMap;

#[derive(Clone, PartialEq, Message)]
struct OciRegistry {
  #[prost(string, tag = "1")]
  reference: String,
  #[prost(message, optional, tag = "2")]
  resolver: Option<RegistryResolver>,
}

#[derive(Clone, PartialEq, Message)]
struct RegistryResolver {
  #[prost(string, tag = "1")]
  auth_stream: String,
  #[prost(map = "string, string", tag = "2")]
  headers: HashMap<String, String>,
}

#[derive(Clone, PartialEq, Message)]
struct ImageStore {
  #[prost(string, tag = "1")]
  name: String,
  #[prost(map = "string, string", tag = "2")]
  labels: HashMap<String, String>,
  #[prost(message, repeated, tag = "10")]
  unpacks: Vec<UnpackConfiguration>,
}

#[derive(Clone, PartialEq, Message)]
struct UnpackConfiguration {
  #[prost(string, tag = "2")]
  snapshotter: String,
}

pub(crate) async fn ensure_oci_image_ready(
  cfg: &ContainerdRuntimeConfig,
  image: &str,
) -> anyhow::Result<()> {
  let client = helper::connect(cfg).await?;
  let ns = cfg.namespace.as_str();

  let mut images = client.images();
  let get = GetImageRequest {
    name: image.to_string(),
  };
  let get = with_namespace!(get, ns);
  if images.get(get).await.is_ok() {
    return Ok(());
  }

  let source = encode_any(
    OciRegistry {
      reference: image.to_string(),
      resolver: None,
    },
    "containerd.types.transfer.OCIRegistry",
  );
  let destination = encode_any(
    ImageStore {
      name: cfg.namespace.clone(),
      labels: HashMap::new(),
      unpacks: vec![UnpackConfiguration {
        snapshotter: cfg.snapshotter.clone(),
      }],
    },
    "containerd.types.transfer.ImageStore",
  );

  let mut transfer = client.transfer();
  let request = TransferRequest {
    source: Some(source),
    destination: Some(destination),
    options: None,
  };
  let request = with_namespace!(request, ns);
  transfer
    .transfer(request)
    .await
    .map_err(|err| anyhow::anyhow!("pull/unpack OCI image {} failed: {}", image, err))?;
  Ok(())
}

fn encode_any<M: Message>(message: M, kind: &str) -> Any {
  Any {
    type_url: format!("/{kind}"),
    value: message.encode_to_vec(),
  }
}
