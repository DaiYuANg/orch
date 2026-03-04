use clap::{Args, Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "xtask", about = "Warden Rust workspace automation")]
pub struct Cli {
  #[command(subcommand)]
  pub command: CommandKind,
}

#[derive(Debug, Subcommand)]
pub enum CommandKind {
  Cluster(ClusterArgs),
  Package(PackageArgs),
  E2e(E2eArgs),
}

#[derive(Debug, Args)]
pub struct ClusterArgs {
  #[command(subcommand)]
  pub command: ClusterCommand,
}

#[derive(Debug, Subcommand)]
pub enum ClusterCommand {
  Run(ClusterRunArgs),
  Status(ClusterStateArgs),
  Stop(ClusterStateArgs),
}

#[derive(Debug, Args)]
pub struct ClusterRunArgs {
  #[arg(long, default_value_t = 3)]
  pub nodes: u16,
  #[arg(long, default_value_t = 7443)]
  pub start_port: u16,
  #[arg(long, default_value = ".tmp/rust-cluster")]
  pub dir: String,
}

#[derive(Debug, Args)]
pub struct ClusterStateArgs {
  #[arg(long, default_value = ".tmp/rust-cluster/state.json")]
  pub state: String,
}

#[derive(Debug, Args)]
pub struct PackageArgs {
  #[arg(long, default_value = "dist/rust")]
  pub out_dir: String,
}

#[derive(Debug, Args)]
pub struct E2eArgs {
  #[arg(long, default_value = "http://127.0.0.1:7443")]
  pub api: String,
  #[arg(long, default_value = "containerd")]
  pub runtime: String,
  #[arg(long, default_value = "xtask-e2e")]
  pub name_prefix: String,
  #[arg(long)]
  pub image: Option<String>,
  #[arg(long, default_value_t = 18080)]
  pub port: u16,
  #[arg(long, default_value_t = 18088)]
  pub ingress_port: u16,
}
