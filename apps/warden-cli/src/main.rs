mod cli_args;
mod dsl_cmd;

use crate::cli_args::{Cli, Command, DslCommand, TaskArgs};
use clap::Parser;
use serde::Serialize;
use std::collections::BTreeMap;
use std::time::Duration;
use warden_client::{WardenClient, parse_endpoint};
use warden_types::{
  BatchActionResult, DeployWorkloadRequest, DnsRecord, EndpointRecord, FailoverRequest,
  MigrateWorkloadRequest, RebalanceRequest, RouteRecord, TaskLogsResponse, WorkloadSummary,
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
    Command::Dsl { cmd } => match cmd {
      DslCommand::Plan(args) => {
        let plan = dsl_cmd::run_plan(&client, &args).await?;
        if args.json {
          print_json(&plan)?;
        } else {
          println!("{}", dsl_cmd::format_plan(&plan));
        }
      }
      DslCommand::Planner(args) => {
        let output = dsl_cmd::run_planner(&args)?;
        print_json(&output)?;
      }
      DslCommand::Render(args) => {
        let compiled = dsl_cmd::run_render(&args)?;
        print_json(&compiled)?;
      }
      DslCommand::Apply(args) => {
        let result = dsl_cmd::run_apply(&client, &args).await?;
        print_json(&result)?;
      }
      DslCommand::Delete(args) => {
        let result = dsl_cmd::run_delete(&client, &args).await?;
        print_json(&result)?;
      }
    },
    Command::Deploy(args) => {
      let process_env = parse_process_env(&args.process_env)?;
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
        process_command: args.process_command,
        process_args: args.process_args,
        process_env,
        process_cwd: args.process_cwd,
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
      TaskArgs::Logs { id, tail } => {
        let item: TaskLogsResponse = client
          .get(&format!("/tasks/{id}/logs?tail={}", tail.max(1)))
          .await?;
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

fn parse_process_env(entries: &[String]) -> anyhow::Result<BTreeMap<String, String>> {
  let mut envs = BTreeMap::new();
  for raw in entries {
    let value = raw.trim();
    if value.is_empty() {
      continue;
    }
    let Some((key, val)) = value.split_once('=') else {
      anyhow::bail!("invalid --process-env entry: {value} (expected KEY=VALUE)");
    };
    let key = key.trim();
    if key.is_empty() {
      anyhow::bail!("invalid --process-env entry: empty key in {value}");
    }
    envs.insert(key.to_string(), val.to_string());
  }
  Ok(envs)
}
