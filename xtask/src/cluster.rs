use crate::cli::{ClusterRunArgs, ClusterStateArgs};
use crate::util::normalize_for_json;
use anyhow::{Context, bail};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;
use std::process::{Command, Stdio};
use sysinfo::{Pid, ProcessesToUpdate, Signal, System};

#[derive(Debug, Serialize, Deserialize)]
struct ClusterState {
  created_at: String,
  nodes: Vec<NodeProcess>,
  config_dir: String,
  log_dir: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct NodeProcess {
  node: u16,
  pid: u32,
  http_port: u16,
  config_path: String,
  log_path: String,
}

pub fn run_cluster(root: &Path, args: &ClusterRunArgs) -> anyhow::Result<()> {
  if args.nodes == 0 {
    bail!("nodes must be greater than 0");
  }
  let runtime_dir = root.join(&args.dir);
  let state_file = runtime_dir.join("state.json");
  if state_file.exists() {
    bail!(
      "cluster state already exists at {} (run `cargo xtask cluster stop` first)",
      state_file.display()
    );
  }

  let config_dir = runtime_dir.join("configs");
  let log_dir = runtime_dir.join("logs");
  fs::create_dir_all(&config_dir).with_context(|| format!("create {}", config_dir.display()))?;
  fs::create_dir_all(&log_dir).with_context(|| format!("create {}", log_dir.display()))?;

  let mut entries = Vec::with_capacity(args.nodes as usize);
  for idx in 0..args.nodes {
    let node = idx + 1;
    let http_port = args.start_port + idx;
    let config_path = config_dir.join(format!("node{node}.yaml"));
    let log_path = log_dir.join(format!("node{node}.log"));
    let socket_path = runtime_dir.join(format!("node{node}.sock"));

    write_node_config(
      &config_path,
      http_port,
      1053 + idx,
      8088 + idx,
      if cfg!(unix) {
        Some(socket_path.to_string_lossy().to_string())
      } else {
        None
      },
    )?;

    let log_file = fs::File::create(&log_path)
      .with_context(|| format!("create log file {}", log_path.display()))?;
    let err_log_file = log_file
      .try_clone()
      .with_context(|| format!("clone log file {}", log_path.display()))?;

    let child = Command::new("cargo")
      .current_dir(root)
      .args(["run", "-p", "warden-server-rs", "--", "--conf"])
      .arg(&config_path)
      .stdin(Stdio::null())
      .stdout(Stdio::from(log_file))
      .stderr(Stdio::from(err_log_file))
      .spawn()
      .with_context(|| format!("spawn node{node} process"))?;

    entries.push(NodeProcess {
      node,
      pid: child.id(),
      http_port,
      config_path: normalize_for_json(&config_path),
      log_path: normalize_for_json(&log_path),
    });
    drop(child);
  }

  let state = ClusterState {
    created_at: chrono::Utc::now().to_rfc3339(),
    nodes: entries,
    config_dir: normalize_for_json(&config_dir),
    log_dir: normalize_for_json(&log_dir),
  };
  fs::write(&state_file, serde_json::to_string_pretty(&state)?)
    .with_context(|| format!("write state file {}", state_file.display()))?;

  println!("cluster started:");
  for item in &state.nodes {
    println!(
      "  node{} pid={} api=http://127.0.0.1:{} log={}",
      item.node, item.pid, item.http_port, item.log_path
    );
  }
  println!("state: {}", state_file.display());
  println!("stop:  cargo xtask cluster stop");
  Ok(())
}

pub fn cluster_status(root: &Path, args: &ClusterStateArgs) -> anyhow::Result<()> {
  let state_path = root.join(&args.state);
  let state = read_state(&state_path)?;
  let mut system = System::new_all();
  system.refresh_processes(ProcessesToUpdate::All, true);

  println!("cluster state: {}", state_path.display());
  for item in &state.nodes {
    let running = system.process(Pid::from_u32(item.pid)).is_some();
    println!(
      "  node{} pid={} running={} api=http://127.0.0.1:{}",
      item.node, item.pid, running, item.http_port
    );
  }
  Ok(())
}

pub fn cluster_stop(root: &Path, args: &ClusterStateArgs) -> anyhow::Result<()> {
  let state_path = root.join(&args.state);
  let state = read_state(&state_path)?;
  let mut system = System::new_all();
  system.refresh_processes(ProcessesToUpdate::All, true);

  for item in &state.nodes {
    if let Some(process) = system.process(Pid::from_u32(item.pid)) {
      let stopped = process.kill_with(Signal::Term).unwrap_or(false) || process.kill();
      println!("stop node{} pid={} result={}", item.node, item.pid, stopped);
    } else {
      println!(
        "stop node{} pid={} result=already-exited",
        item.node, item.pid
      );
    }
  }

  let _ = fs::remove_file(&state_path);
  println!("cluster stopped");
  Ok(())
}

fn read_state(path: &Path) -> anyhow::Result<ClusterState> {
  let text = fs::read_to_string(path).with_context(|| format!("read {}", path.display()))?;
  serde_json::from_str(&text).with_context(|| format!("decode {}", path.display()))
}

fn write_node_config(
  path: &Path,
  http_port: u16,
  dns_port: u16,
  ingress_port: u16,
  unix_socket: Option<String>,
) -> anyhow::Result<()> {
  let socket = unix_socket.unwrap_or_default();
  let yaml = format!(
    "http:\n  port: {http_port}\n  unix_socket: \"{socket}\"\nnetwork:\n  dns_listen: \":{dns_port}\"\n  ingress_http_listen: \":{ingress_port}\"\nlogger:\n  level: \"info\"\n  console: true\n"
  );
  fs::write(path, yaml).with_context(|| format!("write {}", path.display()))
}
