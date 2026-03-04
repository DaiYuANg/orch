use crate::ApiState;
use axum::{Json, extract::State};
use std::collections::{HashMap, HashSet};
use sysinfo::{Disks, System};
use warden_types::{
  ApiEnvelope, ClusterInfo, ClusterNodeInfo, CpuInfo, DiskInfo, MemInfo, RuntimeInfo,
  RuntimeManagedEntry, SystemInfo,
};

#[utoipa::path(get, path = "/system/info", responses((status = 200, description = "System information")))]
pub(crate) async fn system_info() -> Json<ApiEnvelope<SystemInfo>> {
  let mut sys = System::new_all();
  sys.refresh_all();
  Json(ApiEnvelope::ok(SystemInfo {
    hostname: System::host_name().unwrap_or_else(|| String::from("unknown")),
    uptime: System::uptime(),
    os: System::name().unwrap_or_else(|| String::from("unknown")),
    platform: System::long_os_version().unwrap_or_else(|| String::from("unknown")),
    kernel_version: System::kernel_version().unwrap_or_else(|| String::from("unknown")),
    kernel_arch: System::cpu_arch(),
  }))
}

#[utoipa::path(get, path = "/system/cluster", responses((status = 200, description = "Cluster summary")))]
pub(crate) async fn cluster_info(State(state): State<ApiState>) -> Json<ApiEnvelope<ClusterInfo>> {
  let workloads = state.registry.list_workloads().await;
  let endpoints = state.registry.list_endpoints().await;
  let local_node = format!("node-{}", state.raft_node_id);

  let mut workload_count: HashMap<String, usize> = HashMap::new();
  let mut endpoint_count: HashMap<String, usize> = HashMap::new();
  let mut runtime_set: HashMap<String, HashSet<String>> = HashMap::new();
  for item in &workloads {
    *workload_count.entry(item.node_id.clone()).or_insert(0) += 1;
    runtime_set
      .entry(item.node_id.clone())
      .or_default()
      .insert(item.runtime.clone());
  }
  for item in &endpoints {
    *endpoint_count.entry(item.node_id.clone()).or_insert(0) += 1;
  }

  let mut all_nodes = workload_count
    .keys()
    .chain(endpoint_count.keys())
    .cloned()
    .collect::<HashSet<_>>();
  all_nodes.insert(local_node.clone());

  let mut nodes = all_nodes
    .into_iter()
    .map(|node_id| ClusterNodeInfo {
      workloads: *workload_count.get(&node_id).unwrap_or(&0),
      endpoints: *endpoint_count.get(&node_id).unwrap_or(&0),
      runtimes: runtime_set
        .get(&node_id)
        .map(|items| {
          let mut rows = items.iter().cloned().collect::<Vec<_>>();
          rows.sort();
          rows
        })
        .unwrap_or_default(),
      healthy: true,
      node_id,
    })
    .collect::<Vec<_>>();
  nodes.sort_by(|a, b| a.node_id.cmp(&b.node_id));

  let summary = ClusterInfo {
    raft_enabled: state.raft_enabled,
    raft_node_id: state.raft_node_id,
    raft_bind_addr: state.raft_bind_addr,
    leader_node: if state.raft_enabled {
      Some(local_node)
    } else {
      None
    },
    total_nodes: nodes.len(),
    total_workloads: workloads.len(),
    nodes,
  };
  Json(ApiEnvelope::ok(summary))
}

#[utoipa::path(get, path = "/system/cpu", responses((status = 200, description = "CPU information")))]
pub(crate) async fn cpu_info() -> Json<ApiEnvelope<CpuInfo>> {
  let mut sys = System::new_all();
  sys.refresh_cpu_usage();
  let cpus = sys.cpus();
  let model = cpus
    .first()
    .map(|cpu| cpu.brand().to_string())
    .unwrap_or_else(|| String::from("unknown"));
  Json(ApiEnvelope::ok(CpuInfo {
    model_name: model,
    cores: cpus.len(),
    usage_percent: sys.global_cpu_usage(),
  }))
}

#[utoipa::path(get, path = "/system/mem", responses((status = 200, description = "Memory information")))]
pub(crate) async fn mem_info() -> Json<ApiEnvelope<MemInfo>> {
  let mut sys = System::new_all();
  sys.refresh_memory();
  let total = sys.total_memory();
  let used = sys.used_memory();
  let free = total.saturating_sub(used);
  let used_percent = if total == 0 {
    0.0
  } else {
    (used as f64 / total as f64 * 100.0) as f32
  };
  Json(ApiEnvelope::ok(MemInfo {
    total,
    used,
    free,
    used_percent,
  }))
}

#[utoipa::path(get, path = "/system/disk", responses((status = 200, description = "Disk information")))]
pub(crate) async fn disk_info() -> Json<ApiEnvelope<Vec<DiskInfo>>> {
  let disks = Disks::new_with_refreshed_list();
  let rows = disks
    .list()
    .iter()
    .map(|disk| {
      let total = disk.total_space();
      let free = disk.available_space();
      let used = total.saturating_sub(free);
      let used_percent = if total == 0 {
        0.0
      } else {
        (used as f64 / total as f64 * 100.0) as f32
      };
      DiskInfo {
        device: disk.name().to_string_lossy().to_string(),
        mountpoint: disk.mount_point().to_string_lossy().to_string(),
        total,
        used,
        free,
        used_percent,
      }
    })
    .collect::<Vec<_>>();
  Json(ApiEnvelope::ok(rows))
}

#[utoipa::path(get, path = "/system/runtime", responses((status = 200, description = "Runtime providers and managed workloads")))]
pub(crate) async fn runtime_info(State(state): State<ApiState>) -> Json<ApiEnvelope<RuntimeInfo>> {
  let providers = state.task.runtime_providers().await;
  let managed = state
    .task
    .runtime_managed()
    .await
    .into_iter()
    .map(|(workload_id, runtime)| RuntimeManagedEntry {
      workload_id,
      runtime,
    })
    .collect();
  Json(ApiEnvelope::ok(RuntimeInfo { providers, managed }))
}
