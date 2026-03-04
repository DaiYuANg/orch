use anyhow::Context;
use async_trait::async_trait;
use bollard::Docker;
use bollard::errors::Error as DockerError;
use bollard::models::{ContainerCreateBody, HostConfig, PortBinding};
use bollard::query_parameters::{
    CreateContainerOptionsBuilder, CreateImageOptionsBuilder, RemoveContainerOptionsBuilder,
    StartContainerOptions, StopContainerOptionsBuilder,
};
use futures_util::StreamExt;
use std::collections::HashMap;
use tracing::{info, warn};
use warden_runtime::{RuntimeLaunchResult, RuntimeProvider};
use warden_types::DeployWorkloadRequest;

#[derive(Debug, Default)]
pub struct DockerRuntimeProvider;

impl DockerRuntimeProvider {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl RuntimeProvider for DockerRuntimeProvider {
    fn name(&self) -> &'static str {
        "docker"
    }

    async fn start(&self) -> anyhow::Result<()> {
        info!(target: "warden::runtime::docker", "docker runtime provider startup");
        Ok(())
    }

    async fn deploy(
        &self,
        workload_id: &str,
        req: &DeployWorkloadRequest,
    ) -> anyhow::Result<RuntimeLaunchResult> {
        let image = req
            .image
            .as_deref()
            .map(str::trim)
            .filter(|v| !v.is_empty())
            .unwrap_or("nginx:stable-alpine")
            .to_string();
        let service_port = req.service_port.unwrap_or(80);
        let backend = req
            .backend
            .clone()
            .unwrap_or_else(|| format!("127.0.0.1:{service_port}"));
        let container_name = docker_container_name(workload_id);

        let docker = Docker::connect_with_local_defaults().context("connect docker daemon")?;
        pull_docker_image(&docker, &image).await?;

        if let Err(err) = remove_container_if_exists(&docker, &container_name).await {
            warn!(
                target: "warden::runtime::docker",
                workload_id = %workload_id,
                container = %container_name,
                error = %err,
                "remove existing container failed before deploy"
            );
        }

        let exposed_key = format!("{service_port}/tcp");
        let mut exposed_ports = HashMap::new();
        exposed_ports.insert(exposed_key.clone(), HashMap::new());

        let mut port_bindings = HashMap::new();
        port_bindings.insert(
            exposed_key,
            Some(vec![PortBinding {
                host_ip: Some(String::from("0.0.0.0")),
                host_port: Some(service_port.to_string()),
            }]),
        );

        let host_config = HostConfig {
            port_bindings: Some(port_bindings),
            ..Default::default()
        };

        let container_config = ContainerCreateBody {
            image: Some(image.clone()),
            host_config: Some(host_config),
            exposed_ports: Some(exposed_ports),
            ..Default::default()
        };
        let create_options = CreateContainerOptionsBuilder::new()
            .name(&container_name)
            .build();

        docker
            .create_container(Some(create_options), container_config)
            .await
            .with_context(|| format!("create docker container {container_name}"))?;

        docker
            .start_container(&container_name, None::<StartContainerOptions>)
            .await
            .with_context(|| format!("start docker container {container_name}"))?;

        info!(
            target: "warden::runtime::docker",
            workload_id = %workload_id,
            image = %image,
            container = %container_name,
            backend = %backend,
            "docker workload deployed"
        );

        Ok(RuntimeLaunchResult {
            backend_address: backend,
        })
    }

    async fn stop(&self, workload_id: &str) -> anyhow::Result<()> {
        let container_name = docker_container_name(workload_id);
        let docker = Docker::connect_with_local_defaults().context("connect docker daemon")?;

        let stop_result = docker
            .stop_container(
                &container_name,
                Some(StopContainerOptionsBuilder::new().t(8).build()),
            )
            .await;
        match stop_result {
            Ok(_) => {}
            Err(err) if is_not_found_error(&err) => {}
            Err(err) => {
                return Err(anyhow::anyhow!(
                    "stop docker container {} failed: {}",
                    container_name,
                    err
                ));
            }
        }

        remove_container_if_exists(&docker, &container_name).await?;
        info!(
            target: "warden::runtime::docker",
            container = %container_name,
            "docker container stopped"
        );
        Ok(())
    }
}

async fn pull_docker_image(docker: &Docker, image: &str) -> anyhow::Result<()> {
    let options = CreateImageOptionsBuilder::new().from_image(image).build();
    let mut stream = docker.create_image(Some(options), None, None);
    while let Some(item) = stream.next().await {
        if let Err(err) = item {
            return Err(anyhow::anyhow!(
                "pull docker image {} failed: {}",
                image,
                err
            ));
        }
    }
    Ok(())
}

async fn remove_container_if_exists(docker: &Docker, name: &str) -> anyhow::Result<()> {
    let result = docker
        .remove_container(
            name,
            Some(RemoveContainerOptionsBuilder::new().force(true).build()),
        )
        .await;
    match result {
        Ok(_) => Ok(()),
        Err(err) if is_not_found_error(&err) => Ok(()),
        Err(err) => Err(anyhow::anyhow!(
            "remove docker container {} failed: {}",
            name,
            err
        )),
    }
}

fn is_not_found_error(err: &DockerError) -> bool {
    match err {
        DockerError::DockerResponseServerError { status_code, .. } => *status_code == 404,
        _ => false,
    }
}

fn docker_container_name(workload_id: &str) -> String {
    let trimmed = workload_id.trim();
    let normalized = trimmed
        .chars()
        .map(|ch| {
            if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
                ch.to_ascii_lowercase()
            } else {
                '-'
            }
        })
        .collect::<String>();
    format!("warden-{normalized}")
}
