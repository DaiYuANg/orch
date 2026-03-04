use clap::Parser;
use std::sync::Arc;
use tracing::info;
use warden_api::{ApiState, router};
use warden_config::{load, parse_conf_args};
use warden_dns::DnsService;
use warden_ingress::IngressService;
use warden_raft::RaftService;
use warden_registry::RegistryService;
use warden_runtime::RuntimeEngine;
use warden_runtime_containerd::ContainerdRuntimeProvider;
use warden_runtime_docker::DockerRuntimeProvider;
use warden_runtime_firecracker::FirecrackerRuntimeProvider;
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

    let store = new_store(&cfg.store.engine, &cfg.store.path)?;
    store.seed_demo_data().await?;

    let registry = RegistryService::new(store.clone());
    let dns = DnsService::new(store.clone());
    let ingress = IngressService::new(cfg.network.ingress_http_listen.clone(), registry.clone());
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
    let task = TaskService::new(runtime, store.clone());
    let raft = RaftService::new(false, 1, String::from("127.0.0.1:12000"))?;

    dns.start(&cfg.network.dns_listen).await?;
    ingress.start().await;
    task.start().await;
    raft.start().await;

    let app = router(ApiState {
        registry,
        dns,
        task,
    });
    warden_http::run(&cfg, app).await
}
