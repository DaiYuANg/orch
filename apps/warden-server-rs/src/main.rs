use clap::Parser;
use std::sync::Arc;
use std::time::Duration;
use tracing::info;
use warden_api::{ApiState, router};
use warden_config::{load, parse_conf_args};
use warden_dns::{DnsOptions, DnsService};
use warden_ingress::{IngressOptions, IngressService};
use warden_raft::RaftService;
use warden_registry::RegistryService;
use warden_runtime::RuntimeEngine;
use warden_runtime_containerd::ContainerdRuntimeProvider;
use warden_runtime_docker::DockerRuntimeProvider;
use warden_runtime_firecracker::FirecrackerRuntimeProvider;
use warden_runtime_process::ProcessRuntimeProvider;
use warden_store::new_store;
use warden_task::TaskService;

#[derive(Debug, Parser)]
#[command(name = "warden-server-rs", about = "Warden Rust control plane server")]
struct Args {
  #[arg(long = "conf")]
  conf: Vec<String>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
  let args = Args::parse();
  let conf_files = parse_conf_args(&args.conf);
  let cfg = load(&conf_files)?;

  warden_logger::init(&cfg.logger);
  info!(target: "warden::server", files = ?conf_files, "config loaded");
  info!(
    target: "warden::server",
    http_port = cfg.http.port,
    dns_listen = %cfg.network.dns_listen,
    ingress_listen = %cfg.network.ingress_http_listen,
    raft_enabled = cfg.raft.enable,
    raft_node_id = cfg.raft.node_id,
    "server startup begin"
  );

  let store = new_store(&cfg.store.engine, &cfg.store.path)?;
  info!(
    target: "warden::server",
    store_engine = %cfg.store.engine,
    store_path = %cfg.store.path,
    "state store initialized"
  );
  store.seed_demo_data().await?;
  info!(target: "warden::server", "demo seed completed");

  let registry = RegistryService::new(store.clone());
  let dns = DnsService::with_options(
    store.clone(),
    DnsOptions {
      record_cache_capacity: 256,
      record_cache_ttl: Duration::from_millis(cfg.timeouts.dns_record_cache_ttl_ms),
    },
  );
  let ingress = IngressService::with_options(
    cfg.network.ingress_http_listen.clone(),
    registry.clone(),
    IngressOptions {
      http_cache_capacity: 2048,
      http_cache_ttl: Duration::from_millis(cfg.timeouts.ingress_http_cache_ttl_ms),
      http_proxy_timeout: Duration::from_millis(cfg.timeouts.ingress_http_proxy_timeout_ms),
      max_request_body: 10 * 1024 * 1024,
      stream_sync_interval: Duration::from_millis(cfg.timeouts.ingress_stream_sync_interval_ms),
      udp_backend_timeout: Duration::from_millis(cfg.timeouts.ingress_udp_backend_timeout_ms),
    },
  );
  let runtime = RuntimeEngine::new();
  runtime
    .register_provider(Arc::new(DockerRuntimeProvider::new()))
    .await;
  runtime
    .register_provider(Arc::new(ContainerdRuntimeProvider::new()))
    .await;
  runtime
    .register_provider(Arc::new(FirecrackerRuntimeProvider::new()))
    .await;
  runtime
    .register_provider(Arc::new(ProcessRuntimeProvider::new()))
    .await;
  info!(target: "warden::server", "runtime providers registered");
  let raft = RaftService::new(
    cfg.raft.enable,
    cfg.raft.node_id,
    cfg.raft.bind_addr.clone(),
  )?;
  let local_node = format!("node-{}", cfg.raft.node_id);
  let task = TaskService::with_raft(
    runtime,
    store.clone(),
    raft.clone(),
    local_node,
    cfg.raft.worker_nodes.clone(),
  );

  info!(target: "warden::server", listen = %cfg.network.dns_listen, "starting dns service");
  dns.start(&cfg.network.dns_listen).await?;
  info!(target: "warden::server", listen = %cfg.network.ingress_http_listen, "starting ingress service");
  ingress.start().await;
  info!(target: "warden::server", "starting task service");
  task.start().await;
  info!(target: "warden::server", "starting raft service");
  raft.start().await;

  let app = router(ApiState {
    registry,
    dns,
    task,
    raft,
  });
  info!(target: "warden::server", "starting http service");
  warden_http::run(&cfg, app).await
}
