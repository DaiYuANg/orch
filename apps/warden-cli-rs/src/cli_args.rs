use clap::{Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "warden-cli-rs", about = "Warden Rust CLI")]
pub struct Cli {
  #[arg(
    long,
    default_value_t = default_api(),
    help = "api endpoint: auto | unix://... | npipe://... | http(s)://..."
  )]
  pub api: String,
  #[arg(long)]
  pub token: Option<String>,
  #[arg(long, default_value_t = 10)]
  pub timeout_sec: u64,
  #[command(subcommand)]
  pub cmd: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
  Workloads,
  Endpoints,
  Routes,
  Dns,
  Deploy(DeployArgs),
  Stop(StopArgs),
  Migrate(MigrateArgs),
  Failover(FailoverArgs),
  Rebalance(RebalanceArgs),
  Task {
    #[command(subcommand)]
    cmd: TaskArgs,
  },
}

#[derive(Debug, Parser)]
pub struct DeployArgs {
  #[arg(long)]
  pub name: String,
  #[arg(long, default_value = "docker")]
  pub runtime: String,
  #[arg(long)]
  pub image: Option<String>,
  #[arg(long)]
  pub firecracker_config: Option<String>,
  #[arg(long)]
  pub firecracker_kernel_image: Option<String>,
  #[arg(long)]
  pub firecracker_rootfs: Option<String>,
  #[arg(long)]
  pub host: Option<String>,
  #[arg(long, default_value = "/")]
  pub path: String,
  #[arg(long, default_value_t = 80)]
  pub port: u16,
  #[arg(long, default_value_t = 8088)]
  pub ingress_port: u16,
  #[arg(long)]
  pub backend: Option<String>,
}

#[derive(Debug, Parser)]
pub struct StopArgs {
  pub id: String,
}

#[derive(Debug, Parser)]
pub struct MigrateArgs {
  pub id: String,
  #[arg(long)]
  pub target_node: String,
  #[arg(long, default_value_t = false)]
  pub force_stateful: bool,
  #[arg(long, default_value_t = 1)]
  pub max_unavailable: u32,
}

#[derive(Debug, Parser)]
pub struct FailoverArgs {
  #[arg(long)]
  pub failed_node: String,
  #[arg(long)]
  pub target_node: Option<String>,
  #[arg(long, default_value_t = false)]
  pub force_stateful: bool,
  #[arg(long, default_value_t = 1)]
  pub max_unavailable: u32,
  #[arg(long)]
  pub max_migrations: Option<usize>,
}

#[derive(Debug, Parser)]
pub struct RebalanceArgs {
  #[arg(long, default_value_t = 1)]
  pub max_migrations: usize,
}

#[derive(Debug, Subcommand)]
pub enum TaskArgs {
  List,
  Get { id: String },
}

fn default_api() -> String {
  String::from("auto")
}
