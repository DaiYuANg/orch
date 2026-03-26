use anyhow::{Context, bail};
use serde::de::DeserializeOwned;
use warden_types::ApiEnvelope;

pub(crate) fn parse_http_method(method: &str) -> anyhow::Result<reqwest::Method> {
  reqwest::Method::from_bytes(method.as_bytes())
    .with_context(|| format!("invalid http method: {method}"))
}

pub(crate) async fn decode_response<T: DeserializeOwned>(
  resp: reqwest::Response,
) -> anyhow::Result<T> {
  let status = resp.status();
  let body = resp.text().await.context("read response body")?;

  if let Ok(envelope) = serde_json::from_str::<ApiEnvelope<serde_json::Value>>(&body)
    && (!status.is_success() || envelope.code != 0)
  {
    bail!(
      "api error: status={} code={} message={}",
      status.as_u16(),
      envelope.code,
      envelope.message
    );
  }

  if !status.is_success() {
    bail!("request failed: status={} body={}", status.as_u16(), body);
  }

  let envelope: ApiEnvelope<T> = serde_json::from_str(&body).context("decode api envelope")?;
  if envelope.code != 0 {
    bail!(
      "api error: code={} message={}",
      envelope.code,
      envelope.message
    );
  }
  Ok(envelope.data)
}
