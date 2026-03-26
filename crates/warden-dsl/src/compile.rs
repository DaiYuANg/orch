use crate::model::{ApplicationManifest, non_empty};
use serde::{Deserialize, Serialize};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledManifest {
  pub application: String,
  pub namespace: String,
  pub prefix: String,
  pub workloads: Vec<CompiledWorkload>,
  pub ingress_routes: Vec<CompiledIngressRoute>,
  pub warnings: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledWorkload {
  pub name: String,
  pub request: DeployWorkloadRequest,
  pub depends_on: Vec<String>,
  pub warnings: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompiledIngressRoute {
  pub id: String,
  pub protocol: String,
  pub host: String,
  pub path_prefix: String,
  pub listen_port: u16,
  pub backend_workload_name: String,
  pub backend_endpoint_name: String,
  pub dns_enabled: bool,
  pub dns_ttl: u32,
}

pub fn compile_manifest(manifest: &ApplicationManifest) -> anyhow::Result<CompiledManifest> {
  manifest.validate()?;
  let namespace = manifest.namespace().to_string();
  let application = manifest.application().to_string();
  let prefix = format!("{namespace}.{application}.");

  let workloads = manifest
    .spec
    .workloads
    .iter()
    .map(|workload| compile_workload(&namespace, &application, workload))
    .collect::<Vec<_>>();
  let warnings = workloads
    .iter()
    .flat_map(|item| item.warnings.iter().cloned())
    .collect::<Vec<_>>();
  let ingress_routes = manifest
    .spec
    .workloads
    .iter()
    .filter_map(|workload| compile_ingress_route(&namespace, &application, workload))
    .collect::<Vec<_>>();

  Ok(CompiledManifest {
    application,
    namespace,
    prefix,
    workloads,
    ingress_routes,
    warnings: sorted_unique(warnings),
  })
}

fn compile_workload(
  namespace: &str,
  application: &str,
  workload: &crate::model::WorkloadSpec,
) -> CompiledWorkload {
  let name = format!("{namespace}.{application}.{}", workload.name.trim());
  let service_port = workload.service.as_ref().and_then(|v| v.port).unwrap_or(80);
  let backend = workload
    .service
    .as_ref()
    .and_then(|v| non_empty(v.backend.as_deref()));
  let ingress_path = workload
    .ingress
    .as_ref()
    .and_then(|v| non_empty(v.path.as_deref()))
    .unwrap_or_else(|| String::from("/"));
  let ingress_port = workload
    .ingress
    .as_ref()
    .and_then(|v| v.listen_port)
    .unwrap_or(8088);
  let host = workload
    .ingress
    .as_ref()
    .and_then(|v| non_empty(v.host.as_deref()))
    .unwrap_or_else(|| format!("{name}.warden.local"));
  let image = non_empty(workload.image.as_deref());
  let process = workload.process.as_ref();
  let firecracker = workload.firecracker.as_ref();

  let request = DeployWorkloadRequest {
    name: name.clone(),
    runtime: workload.runtime.trim().to_string(),
    image,
    firecracker_config: firecracker.and_then(|v| non_empty(v.config.as_deref())),
    firecracker_kernel_image: firecracker.and_then(|v| non_empty(v.kernel_image.as_deref())),
    firecracker_rootfs: firecracker.and_then(|v| non_empty(v.rootfs.as_deref())),
    host: Some(host),
    path_prefix: Some(ingress_path),
    service_port: Some(service_port),
    ingress_port: Some(ingress_port),
    backend,
    process_command: process.and_then(|v| non_empty(v.command.as_deref())),
    process_args: process
      .map(|v| {
        v.args
          .iter()
          .map(|item| item.trim())
          .filter(|item| !item.is_empty())
          .map(ToString::to_string)
          .collect::<Vec<_>>()
      })
      .unwrap_or_default(),
    process_env: process.map(|v| v.env.clone()).unwrap_or_default(),
    process_cwd: process.and_then(|v| non_empty(v.cwd.as_deref())),
  };
  let depends_on = workload
    .depends_on
    .iter()
    .map(|dep| format!("{namespace}.{application}.{}", dep.trim()))
    .collect::<Vec<_>>();

  let mut warnings = Vec::new();
  if workload.scheduling.is_some() {
    warnings.push(format!(
      "workload {} sets scheduling, but current deploy api does not expose scheduling policy",
      name
    ));
  }
  if workload.replicas.is_some() {
    warnings.push(format!(
      "workload {} sets replicas, but current deploy api does not expose replica count",
      name
    ));
  }
  CompiledWorkload {
    name,
    request,
    depends_on,
    warnings: sorted_unique(warnings),
  }
}

fn compile_ingress_route(
  namespace: &str,
  application: &str,
  workload: &crate::model::WorkloadSpec,
) -> Option<CompiledIngressRoute> {
  let ingress = workload.ingress.as_ref()?;
  if matches!(ingress.enabled, Some(false)) {
    return None;
  }

  let workload_name = format!("{namespace}.{application}.{}", workload.name.trim());
  let host =
    non_empty(ingress.host.as_deref()).unwrap_or_else(|| format!("{workload_name}.warden.local"));
  let path_prefix = non_empty(ingress.path.as_deref()).unwrap_or_else(|| String::from("/"));
  let listen_port = ingress.listen_port.unwrap_or(8088);
  let dns_enabled = workload
    .dns
    .as_ref()
    .and_then(|v| v.enabled)
    .unwrap_or(true);
  let dns_ttl = workload.dns.as_ref().and_then(|v| v.ttl).unwrap_or(60);

  Some(CompiledIngressRoute {
    id: format!("route-{workload_name}"),
    protocol: String::from("http"),
    host,
    path_prefix,
    listen_port,
    backend_workload_name: workload_name,
    backend_endpoint_name: String::from("http"),
    dns_enabled,
    dns_ttl,
  })
}

fn sorted_unique(mut items: Vec<String>) -> Vec<String> {
  items.sort();
  items.dedup();
  items
}
