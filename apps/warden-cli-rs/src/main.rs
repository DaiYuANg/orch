mod cli_args;

use crate::cli_args::{Cli, Command, TaskArgs};
use clap::Parser;
use serde::Serialize;
use std::time::Duration;
use warden_client::{WardenClient, parse_endpoint};
use warden_types::{
  BatchActionResult, DeployWorkloadRequest, DnsRecord, EndpointRecord, FailoverRequest,
  MigrateWorkloadRequest, RebalanceRequest, RouteRecord, WorkloadSummary,
};

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
        firecracker_config: args.firecracker_config,
        firecracker_kernel_image: args.firecracker_kernel_image,
        firecracker_rootfs: args.firecracker_rootfs,
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

fn print_json<T: Serialize>(value: &T) -> anyhow::Result<()> {
  println!("{}", serde_json::to_string_pretty(value)?);
  Ok(())
}
