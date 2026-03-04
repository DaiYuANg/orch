use clap::{Parser, Subcommand};
use serde::Serialize;
use std::time::Duration;
use warden_client::{WardenClient, parse_endpoint};
use warden_types::{
    DeployWorkloadRequest, DnsRecord, EndpointRecord, RouteRecord, WorkloadSummary,
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
