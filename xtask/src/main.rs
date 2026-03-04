mod cli;
mod cluster;
mod e2e;
mod package;
mod util;

use clap::Parser;
use cli::{Cli, ClusterCommand, CommandKind};
use util::workspace_root;

fn main() -> anyhow::Result<()> {
  let cli = Cli::parse();
  let root = workspace_root()?;
  match cli.command {
    CommandKind::Cluster(cluster) => match cluster.command {
      ClusterCommand::Run(args) => cluster::run_cluster(&root, &args),
      ClusterCommand::Status(args) => cluster::cluster_status(&root, &args),
      ClusterCommand::Stop(args) => cluster::cluster_stop(&root, &args),
    },
    CommandKind::Package(args) => package::package_release(&root, &args),
    CommandKind::E2e(args) => e2e::run_e2e(&root, &args),
  }
}
