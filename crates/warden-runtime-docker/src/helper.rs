use bollard::Docker;
use bollard::errors::Error as DockerError;
use bollard::query_parameters::{CreateImageOptionsBuilder, RemoveContainerOptionsBuilder};
use futures_util::StreamExt;

pub(crate) async fn pull_docker_image(docker: &Docker, image: &str) -> anyhow::Result<()> {
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

pub(crate) async fn remove_container_if_exists(docker: &Docker, name: &str) -> anyhow::Result<()> {
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

pub(crate) fn is_not_found_error(err: &DockerError) -> bool {
  match err {
    DockerError::DockerResponseServerError { status_code, .. } => *status_code == 404,
    _ => false,
  }
}

pub(crate) fn docker_container_name(workload_id: &str) -> String {
  let normalized = workload_id
    .trim()
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
