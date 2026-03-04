use clap::{Parser, Subcommand};
use serde::Serialize;
use std::time::Duration;
use warden_client::{WardenClient, parse_endpoint};
use warden_types::{
  BatchActionResult, DeployWorkloadRequest, DnsRecord, EndpointRecord, FailoverRequest,
  MigrateWorkloadRequest, RebalanceRequest, RouteRecord, WorkloadSummary,
};

#[derive(Debug, Parser)]
#[command(name = "warden-cli-rs", about = "Warden Rust CLI")]
struct Cli {
  #[arg(
        long,
        default_value_t = default_api(),
        help = "api endpoint: auto | unix://... | npipe://... | http(s)://..."
    )]
  api: String,
  #[arg(long)]
  token: Option<String>,
  #[arg(long, default_value_t = 10)]
  timeout_sec: u64,
  #[command(subcommand)]
  cmd: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
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
struct DeployArgs {
  #[arg(long)]
  name: String,
  #[arg(long, default_value = "docker")]
  runtime: String,
  #[arg(long)]
  image: Option<String>,
  #[arg(long)]
  host: Option<String>,
  #[arg(long, default_value = "/")]
  path: String,
  #[arg(long, default_value_t = 80)]
  port: u16,
  #[arg(long, default_value_t = 8088)]
  ingress_port: u16,
  #[arg(long)]
  backend: Option<String>,
}

#[derive(Debug, Parser)]
struct StopArgs {
  id: String,
}

#[derive(Debug, Parser)]
struct MigrateArgs {
  id: String,
  #[arg(long)]
  target_node: String,
  #[arg(long, default_value_t = false)]
  force_stateful: bool,
  #[arg(long, default_value_t = 1)]
  max_unavailable: u32,
}

#[derive(Debug, Parser)]
struct FailoverArgs {
  #[arg(long)]
  failed_node: String,
  #[arg(long)]
  target_node: Option<String>,
  #[arg(long, default_value_t = false)]
  force_stateful: bool,
  #[arg(long, default_value_t = 1)]
  max_unavailable: u32,
  #[arg(long)]
  max_migrations: Option<usize>,
}

#[derive(Debug, Parser)]
struct RebalanceArgs {
  #[arg(long, default_value_t = 1)]
  max_migrations: usize,
}

#[derive(Debug, Subcommand)]
enum TaskArgs {
  List,
  Get { id: String },
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
  let cli = Cli::parse();
  let endpoint = parse_endpoint(&cli.api)?;
  let client = WardenClient::new(endpoint, cli.token, Duration::from_secs(cli.timeout_sec));

  match cli.cmd {
    Command::Workloads => {
      let items: Vec<WorkloadSummary> = client.get("/workloads").await?;
      print_json(&items)?;
    }
    Command::Endpoints => {
      let items: Vec<EndpointRecord> = client.get("/system/endpoints").await?;
      print_json(&items)?;
    }
    Command::Routes => {
      let items: Vec<RouteRecord> = client.get("/system/routes").await?;
      print_json(&items)?;
    }
    Command::Dns => {
      let items: Vec<DnsRecord> = client.get("/system/dns/records").await?;
      print_json(&items)?;
    }
    Command::Deploy(args) => {
      let req = DeployWorkloadRequest {
        name: args.name,
        runtime: args.runtime,
        image: args.image,
        host: args.host,
        path_prefix: Some(args.path),
        service_port: Some(args.port),
        ingress_port: Some(args.ingress_port),
        backend: args.backend,
      };
      let item: WorkloadSummary = client.post("/tasks/deploy", &req).await?;
      print_json(&item)?;
    }
    Command::Stop(args) => {
      let path = format!("/tasks/{}/stop", args.id);
      let item: WorkloadSummary = client.post(&path, &serde_json::json!({})).await?;
      print_json(&item)?;
    }
    Command::Migrate(args) => {
      let req = MigrateWorkloadRequest {
        target_node: args.target_node,
        force_stateful: args.force_stateful,
        max_unavailable: args.max_unavailable,
      };
      let item: WorkloadSummary = client
        .post(&format!("/tasks/{}/migrate", args.id), &req)
        .await?;
      print_json(&item)?;
    }
    Command::Failover(args) => {
      let req = FailoverRequest {
        failed_node: args.failed_node,
        target_node: args.target_node,
        force_stateful: args.force_stateful,
        max_unavailable: args.max_unavailable,
        max_migrations: args.max_migrations,
      };
      let result: BatchActionResult = client.post("/tasks/failover", &req).await?;
      print_json(&result)?;
    }
    Command::Rebalance(args) => {
      let req = RebalanceRequest {
        max_migrations: args.max_migrations,
      };
      let result: BatchActionResult = client.post("/tasks/rebalance", &req).await?;
      print_json(&result)?;
    }
    Command::Task { cmd } => match cmd {
      TaskArgs::List => {
        let items: Vec<WorkloadSummary> = client.get("/tasks").await?;
        print_json(&items)?;
      }
      TaskArgs::Get { id } => {
        let item: WorkloadSummary = client.get(&format!("/tasks/{id}")).await?;
        print_json(&item)?;
      }
    },
  }

  Ok(())
}

fn default_api() -> String {
  String::from("auto")
}

fn print_json<T: Serialize>(value: &T) -> anyhow::Result<()> {
  println!("{}", serde_json::to_string_pretty(value)?);
  Ok(())
}
