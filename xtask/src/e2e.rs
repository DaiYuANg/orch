use crate::cli::E2eArgs;
use crate::util::{run_command, run_command_capture};
use anyhow::Context;
use chrono::Utc;
use serde::Deserialize;
use std::path::Path;

#[derive(Debug, Deserialize)]
struct WorkloadSummary {
  id: String,
  name: String,
}

pub fn run_e2e(root: &Path, args: &E2eArgs) -> anyhow::Result<()> {
  smoke_read_only(root, args)?;

  let deploy_name = format!("{}-{}", args.name_prefix, Utc::now().timestamp());
  let deployed = deploy_workload(root, args, &deploy_name)?;
  if let Err(err) = verify_and_cleanup(root, args, &deployed.id) {
    let _ = stop_workload(root, args, &deployed.id);
    return Err(err);
  }

  println!(
    "e2e deploy-stop finished: runtime={} name={} id={}",
    args.runtime, deployed.name, deployed.id
  );
  Ok(())
}

fn smoke_read_only(root: &Path, args: &E2eArgs) -> anyhow::Result<()> {
  run_cli(root, args, &["workloads"])?;
  run_cli(root, args, &["routes"])?;
  run_cli(root, args, &["dns"])?;
  Ok(())
}

fn verify_and_cleanup(root: &Path, args: &E2eArgs, workload_id: &str) -> anyhow::Result<()> {
  let list_before = list_workloads(root, args)?;
  let exists = list_before.iter().any(|item| item.id == workload_id);
  if !exists {
    return Err(anyhow::anyhow!(
      "deployed workload {} missing from workloads list",
      workload_id
    ));
  }

  stop_workload(root, args, workload_id)?;

  let list_after = list_workloads(root, args)?;
  let still_exists = list_after.iter().any(|item| item.id == workload_id);
  if still_exists {
    return Err(anyhow::anyhow!(
      "workload {} still exists after stop",
      workload_id
    ));
  }
  Ok(())
}

fn deploy_workload(root: &Path, args: &E2eArgs, name: &str) -> anyhow::Result<WorkloadSummary> {
  let mut cmd = vec![
    String::from("deploy"),
    String::from("--name"),
    name.to_string(),
    String::from("--runtime"),
    args.runtime.trim().to_string(),
    String::from("--path"),
    String::from("/"),
    String::from("--port"),
    args.port.to_string(),
    String::from("--ingress-port"),
    args.ingress_port.to_string(),
  ];
  if let Some(image) = args
    .image
    .as_deref()
    .map(str::trim)
    .filter(|v| !v.is_empty())
  {
    cmd.push(String::from("--image"));
    cmd.push(image.to_string());
  }

  run_cli_json(root, args, &cmd)
}

fn stop_workload(root: &Path, args: &E2eArgs, workload_id: &str) -> anyhow::Result<()> {
  run_cli(root, args, &["stop", workload_id])
}

fn list_workloads(root: &Path, args: &E2eArgs) -> anyhow::Result<Vec<WorkloadSummary>> {
  run_cli_json(root, args, &[String::from("workloads")])
}

fn run_cli(root: &Path, args: &E2eArgs, cmd: &[&str]) -> anyhow::Result<()> {
  let mut command = vec![
    "run",
    "--quiet",
    "-p",
    "warden-cli-rs",
    "--",
    "--api",
    args.api.as_str(),
  ];
  command.extend(cmd.iter().copied());
  run_command(root, "cargo", &command)
}

fn run_cli_json<T>(root: &Path, args: &E2eArgs, cmd: &[String]) -> anyhow::Result<T>
where
  T: for<'de> Deserialize<'de>,
{
  let mut command = vec![
    "run".to_string(),
    "--quiet".to_string(),
    "-p".to_string(),
    "warden-cli-rs".to_string(),
    "--".to_string(),
    "--api".to_string(),
    args.api.clone(),
  ];
  command.extend(cmd.iter().cloned());
  let refs = command.iter().map(String::as_str).collect::<Vec<_>>();
  let output = run_command_capture(root, "cargo", &refs)?;

  parse_json_from_output(&output).with_context(|| format!("parse cli output as json: {}", output))
}

fn parse_json_from_output<T>(output: &str) -> anyhow::Result<T>
where
  T: for<'de> Deserialize<'de>,
{
  if let Ok(value) = serde_json::from_str::<T>(output.trim()) {
    return Ok(value);
  }

  for line in output.lines().rev() {
    let trimmed = line.trim();
    if trimmed.is_empty() {
      continue;
    }
    if let Ok(value) = serde_json::from_str::<T>(trimmed) {
      return Ok(value);
    }
  }
  Err(anyhow::anyhow!("no JSON payload found in CLI output"))
}
