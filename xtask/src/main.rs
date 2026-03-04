use anyhow::{Context, bail};
use clap::{Args, Parser, Subcommand};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use sysinfo::{Pid, ProcessesToUpdate, Signal, System};

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    let root = workspace_root()?;
    match cli.command {
        CommandKind::Cluster(cluster) => match cluster.command {
            ClusterCommand::Run(args) => run_cluster(&root, &args),
            ClusterCommand::Status(args) => cluster_status(&root, &args),
            ClusterCommand::Stop(args) => cluster_stop(&root, &args),
        },
        CommandKind::Package(args) => package_release(&root, &args),
        CommandKind::E2e(args) => run_e2e(&root, &args),
    }
}

#[derive(Debug, Parser)]
#[command(name = "xtask", about = "Warden Rust workspace automation")]
struct Cli {
    #[command(subcommand)]
    command: CommandKind,
}

#[derive(Debug, Subcommand)]
enum CommandKind {
    Cluster(ClusterArgs),
    Package(PackageArgs),
    E2e(E2eArgs),
}

#[derive(Debug, Args)]
struct ClusterArgs {
    #[command(subcommand)]
    command: ClusterCommand,
}

#[derive(Debug, Subcommand)]
enum ClusterCommand {
    Run(ClusterRunArgs),
    Status(ClusterStateArgs),
    Stop(ClusterStateArgs),
}

#[derive(Debug, Args)]
struct ClusterRunArgs {
    #[arg(long, default_value_t = 3)]
    nodes: u16,
    #[arg(long, default_value_t = 7443)]
    start_port: u16,
    #[arg(long, default_value = ".tmp/rust-cluster")]
    dir: String,
}

#[derive(Debug, Args)]
struct ClusterStateArgs {
    #[arg(long, default_value = ".tmp/rust-cluster/state.json")]
    state: String,
}

#[derive(Debug, Args)]
struct PackageArgs {
    #[arg(long, default_value = "dist/rust")]
    out_dir: String,
}

#[derive(Debug, Args)]
struct E2eArgs {
    #[arg(long, default_value = "http://127.0.0.1:7443")]
    api: String,
}

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

fn run_cluster(root: &Path, args: &ClusterRunArgs) -> anyhow::Result<()> {
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
        let dns_port = 1053 + idx;
        let ingress_port = 8088 + idx;
        let config_path = config_dir.join(format!("node{node}.yaml"));
        let log_path = log_dir.join(format!("node{node}.log"));
        let socket_path = runtime_dir.join(format!("node{node}.sock"));

        write_node_config(
            &config_path,
            http_port,
            dns_port,
            ingress_port,
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

        let pid = child.id();
        drop(child);

        entries.push(NodeProcess {
            node,
            pid,
            http_port,
            config_path: normalize_for_json(&config_path),
            log_path: normalize_for_json(&log_path),
        });
    }

    let state = ClusterState {
        created_at: chrono::Utc::now().to_rfc3339(),
        nodes: entries,
        config_dir: normalize_for_json(&config_dir),
        log_dir: normalize_for_json(&log_dir),
    };
    let state_json = serde_json::to_string_pretty(&state)?;
    fs::write(&state_file, state_json)
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

fn cluster_status(root: &Path, args: &ClusterStateArgs) -> anyhow::Result<()> {
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

fn cluster_stop(root: &Path, args: &ClusterStateArgs) -> anyhow::Result<()> {
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

fn package_release(root: &Path, args: &PackageArgs) -> anyhow::Result<()> {
    run_command(
        root,
        "cargo",
        &[
            "build",
            "--release",
            "-p",
            "warden-server-rs",
            "-p",
            "warden-cli-rs",
        ],
    )?;

    let out_dir = root.join(&args.out_dir);
    fs::create_dir_all(&out_dir).with_context(|| format!("create {}", out_dir.display()))?;

    let exe_suffix = if cfg!(windows) { ".exe" } else { "" };
    let server_bin = format!("warden-server-rs{exe_suffix}");
    let cli_bin = format!("warden-cli-rs{exe_suffix}");

    let target_dir = root.join("target").join("release");
    copy_file(
        &target_dir.join(&server_bin),
        &out_dir.join(&server_bin),
        "server binary",
    )?;
    copy_file(
        &target_dir.join(&cli_bin),
        &out_dir.join(&cli_bin),
        "cli binary",
    )?;

    println!("package output: {}", out_dir.display());
    Ok(())
}

fn run_e2e(root: &Path, args: &E2eArgs) -> anyhow::Result<()> {
    run_command(
        root,
        "cargo",
        &[
            "run",
            "-p",
            "warden-cli-rs",
            "--",
            "--api",
            &args.api,
            "workloads",
        ],
    )?;
    run_command(
        root,
        "cargo",
        &[
            "run",
            "-p",
            "warden-cli-rs",
            "--",
            "--api",
            &args.api,
            "routes",
        ],
    )?;
    run_command(
        root,
        "cargo",
        &[
            "run",
            "-p",
            "warden-cli-rs",
            "--",
            "--api",
            &args.api,
            "dns",
        ],
    )?;
    println!("e2e smoke finished");
    Ok(())
}

fn run_command(root: &Path, program: &str, args: &[&str]) -> anyhow::Result<()> {
    let status = Command::new(program)
        .current_dir(root)
        .args(args)
        .status()
        .with_context(|| format!("run command: {} {}", program, args.join(" ")))?;
    if !status.success() {
        bail!(
            "command failed with status {}: {} {}",
            status,
            program,
            args.join(" ")
        );
    }
    Ok(())
}

fn read_state(path: &Path) -> anyhow::Result<ClusterState> {
    let text = fs::read_to_string(path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_str(&text).with_context(|| format!("decode {}", path.display()))
}

fn copy_file(src: &Path, dst: &Path, name: &str) -> anyhow::Result<()> {
    if !src.exists() {
        bail!("{} missing: {}", name, src.display());
    }
    fs::copy(src, dst).with_context(|| format!("copy {} -> {}", src.display(), dst.display()))?;
    Ok(())
}

fn normalize_for_json(path: &Path) -> String {
    path.to_string_lossy().replace('\\', "/")
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

fn workspace_root() -> anyhow::Result<PathBuf> {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .map(Path::to_path_buf)
        .context("resolve workspace root from xtask manifest dir")
}
