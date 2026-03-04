use std::collections::BTreeMap;
use tokio::time::{Duration, sleep};
use warden_runtime::RuntimeProvider;
use warden_runtime_process::ProcessRuntimeProvider;
use warden_types::DeployWorkloadRequest;

#[tokio::test]
async fn deploy_and_stop_process_workload() -> anyhow::Result<()> {
  let provider = ProcessRuntimeProvider::new();
  let req = running_process_request();

  let launched = provider.deploy("wk-process-test", &req).await?;
  assert_eq!(launched.backend_address, "127.0.0.1:18081");

  provider.stop("wk-process-test").await?;
  provider.stop("wk-process-test").await?;
  Ok(())
}

#[tokio::test]
async fn deploy_requires_command() {
  let provider = ProcessRuntimeProvider::new();
  let req = DeployWorkloadRequest {
    process_command: None,
    ..running_process_request()
  };

  let result = provider.deploy("wk-process-missing", &req).await;
  assert!(result.is_err());
}

#[tokio::test]
async fn logs_returns_recent_lines() -> anyhow::Result<()> {
  let provider = ProcessRuntimeProvider::new();
  let req = logging_process_request();
  provider.deploy("wk-process-logs", &req).await?;
  sleep(Duration::from_millis(500)).await;

  let lines = provider.logs("wk-process-logs", 10).await?;
  assert!(!lines.is_empty());
  assert!(lines.iter().any(|line| line.contains("warden-process-log")));
  provider.stop("wk-process-logs").await?;
  Ok(())
}

fn running_process_request() -> DeployWorkloadRequest {
  DeployWorkloadRequest {
    name: String::from("proc"),
    runtime: String::from("process"),
    image: None,
    firecracker_config: None,
    firecracker_kernel_image: None,
    firecracker_rootfs: None,
    host: None,
    path_prefix: None,
    service_port: Some(18081),
    ingress_port: Some(18088),
    backend: None,
    process_command: Some(process_command()),
    process_args: process_args(),
    process_env: BTreeMap::new(),
    process_cwd: None,
  }
}

fn logging_process_request() -> DeployWorkloadRequest {
  DeployWorkloadRequest {
    process_command: Some(process_command()),
    process_args: logging_process_args(),
    ..running_process_request()
  }
}

#[cfg(windows)]
fn process_command() -> String {
  String::from("powershell")
}

#[cfg(windows)]
fn process_args() -> Vec<String> {
  vec![
    String::from("-NoProfile"),
    String::from("-Command"),
    String::from("Start-Sleep -Seconds 30"),
  ]
}

#[cfg(not(windows))]
fn logging_process_args() -> Vec<String> {
  vec![
    String::from("-c"),
    String::from(
      "i=0; while [ $i -lt 3 ]; do echo warden-process-log-$i; i=$((i+1)); sleep 0.1; done; sleep 30",
    ),
  ]
}

#[cfg(windows)]
fn logging_process_args() -> Vec<String> {
  vec![
    String::from("-NoProfile"),
    String::from("-Command"),
    String::from(
      "$i=0; while ($i -lt 3) { Write-Output \"warden-process-log-$i\"; Start-Sleep -Milliseconds 100; $i++ }; Start-Sleep -Seconds 30",
    ),
  ]
}

#[cfg(not(windows))]
fn process_command() -> String {
  String::from("sh")
}

#[cfg(not(windows))]
fn process_args() -> Vec<String> {
  vec![String::from("-c"), String::from("sleep 30")]
}
