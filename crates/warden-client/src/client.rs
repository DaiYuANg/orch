use crate::endpoint::{Endpoint, auto_endpoints, describe_endpoint, ensure_path};
use crate::request::{decode_response, parse_http_method};
use crate::windows_npipe::request_via_npipe;
use anyhow::{Context, bail};
use reqwest_middleware::{ClientBuilder, ClientWithMiddleware};
use reqwest_retry::{
  RetryTransientMiddleware,
  policies::{ExponentialBackoff, ExponentialBackoffBuilder},
};
use serde::{Serialize, de::DeserializeOwned};
use std::time::Duration;

#[derive(Debug, Clone)]
pub struct WardenClient {
  endpoint: Endpoint,
  token: Option<String>,
  timeout: Duration,
}

impl WardenClient {
  pub fn new(endpoint: Endpoint, token: Option<String>, timeout: Duration) -> Self {
    Self {
      endpoint,
      token,
      timeout,
    }
  }

  pub async fn get<T: DeserializeOwned>(&self, path: &str) -> anyhow::Result<T> {
    let uri_path = ensure_path(path);
    if let Endpoint::Auto = self.endpoint {
      return self.request_with_fallback("GET", &uri_path, None).await;
    }
    self
      .request_once("GET", &self.endpoint, &uri_path, None)
      .await
  }

  pub async fn post<TReq: Serialize, TResp: DeserializeOwned>(
    &self,
    path: &str,
    body: &TReq,
  ) -> anyhow::Result<TResp> {
    let uri_path = ensure_path(path);
    let payload = serde_json::to_vec(body).context("serialize post body")?;
    if let Endpoint::Auto = self.endpoint {
      return self
        .request_with_fallback("POST", &uri_path, Some(payload))
        .await;
    }
    self
      .request_once("POST", &self.endpoint, &uri_path, Some(payload))
      .await
  }

  async fn request_with_fallback<T: DeserializeOwned>(
    &self,
    method: &str,
    path: &str,
    body: Option<Vec<u8>>,
  ) -> anyhow::Result<T> {
    let mut failures: Vec<String> = Vec::new();
    for endpoint in auto_endpoints() {
      match self
        .request_once(method, &endpoint, path, body.clone())
        .await
      {
        Ok(data) => return Ok(data),
        Err(err) => failures.push(format!("{} => {}", describe_endpoint(&endpoint), err)),
      }
    }
    bail!(
      "all endpoint attempts failed (auto): {}",
      failures.join(" | ")
    )
  }

  async fn request_once<T: DeserializeOwned>(
    &self,
    method: &str,
    endpoint: &Endpoint,
    path: &str,
    body: Option<Vec<u8>>,
  ) -> anyhow::Result<T> {
    let retry_enabled = is_idempotent_method(method);
    match endpoint {
      Endpoint::Http(base) => {
        let url = format!("{}{}", base.trim_end_matches('/'), path);
        let client = self.build_http_client(retry_enabled)?;
        let req = client.request(parse_http_method(method)?, url);
        let req = if let Some(payload) = body {
          req.header("Content-Type", "application/json").body(payload)
        } else {
          req
        };
        let req = attach_auth_mw(req, self.token.as_deref());
        let resp = req.send().await.context("send http request with retry")?;
        decode_response(resp).await
      }
      Endpoint::Unix(socket) => {
        #[cfg(unix)]
        {
          let raw = reqwest::Client::builder()
            .unix_socket(socket)
            .timeout(self.timeout)
            .build()
            .context("build unix socket client")?;
          let client = self.wrap_with_retry(raw, retry_enabled);
          let req = client.request(
            parse_http_method(method)?,
            format!("http://localhost{}", path),
          );
          let req = if let Some(payload) = body {
            req.header("Content-Type", "application/json").body(payload)
          } else {
            req
          };
          let req = attach_auth_mw(req, self.token.as_deref());
          let resp = req.send().await.context("send unix request with retry")?;
          decode_response(resp).await
        }
        #[cfg(not(unix))]
        {
          let _ = socket;
          bail!("unix socket endpoint is not supported on this platform target");
        }
      }
      Endpoint::Npipe(pipe_name) => {
        request_via_npipe(
          pipe_name,
          method,
          path,
          body,
          self.token.as_deref(),
          self.timeout,
        )
        .await
      }
      Endpoint::Auto => {
        bail!("auto endpoint should be resolved before single-attempt request")
      }
    }
  }

  fn build_http_client(&self, retry_enabled: bool) -> anyhow::Result<ClientWithMiddleware> {
    let raw = reqwest::Client::builder().timeout(self.timeout).build()?;
    Ok(self.wrap_with_retry(raw, retry_enabled))
  }

  fn wrap_with_retry(&self, client: reqwest::Client, retry_enabled: bool) -> ClientWithMiddleware {
    let builder = ClientBuilder::new(client);
    if retry_enabled {
      let retry_policy = retry_policy();
      builder
        .with(RetryTransientMiddleware::new_with_policy(retry_policy))
        .build()
    } else {
      builder.build()
    }
  }
}

fn retry_policy() -> ExponentialBackoff {
  ExponentialBackoffBuilder::default().build_with_max_retries(3)
}

fn is_idempotent_method(method: &str) -> bool {
  matches!(method.trim(), "GET" | "HEAD" | "OPTIONS" | "DELETE")
}

fn attach_auth_mw(
  req: reqwest_middleware::RequestBuilder,
  token: Option<&str>,
) -> reqwest_middleware::RequestBuilder {
  if let Some(value) = token.and_then(non_empty) {
    req.bearer_auth(value)
  } else {
    req
  }
}

fn non_empty(v: &str) -> Option<&str> {
  let t = v.trim();
  if t.is_empty() { None } else { Some(t) }
}
