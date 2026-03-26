use anyhow::bail;
#[cfg(windows)]
use anyhow::{Context, anyhow};
use serde::de::DeserializeOwned;
use std::time::Duration;
#[cfg(windows)]
use warden_types::ApiEnvelope;

#[cfg(windows)]
pub(crate) async fn request_via_npipe<T: DeserializeOwned>(
  pipe_name: &str,
  method: &str,
  path: &str,
  body: Option<Vec<u8>>,
  token: Option<&str>,
  timeout_value: Duration,
) -> anyhow::Result<T> {
  use tokio::io::{AsyncReadExt, AsyncWriteExt};
  use tokio::net::windows::named_pipe::ClientOptions;
  use tokio::time::timeout;

  let mut pipe = ClientOptions::new()
    .security_qos_flags(0)
    .read(true)
    .write(true)
    .open(pipe_name)
    .with_context(|| format!("open named pipe {}", pipe_name))?;

  let payload = body.unwrap_or_default();
  let mut request = format!(
    "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\nConnection: close\r\nContent-Length: {}\r\n",
    payload.len()
  );
  if !payload.is_empty() {
    request.push_str("Content-Type: application/json\r\n");
  }
  if let Some(value) = token.and_then(non_empty) {
    request.push_str("Authorization: Bearer ");
    request.push_str(value);
    request.push_str("\r\n");
  }
  request.push_str("\r\n");

  timeout(timeout_value, pipe.write_all(request.as_bytes()))
    .await
    .context("timeout writing named pipe request")?
    .context("write named pipe request failed")?;
  if !payload.is_empty() {
    timeout(timeout_value, pipe.write_all(&payload))
      .await
      .context("timeout writing named pipe body")?
      .context("write named pipe body failed")?;
  }
  timeout(timeout_value, pipe.flush())
    .await
    .context("timeout flushing named pipe request")?
    .context("flush named pipe request failed")?;

  let mut response = Vec::with_capacity(4096);
  timeout(timeout_value, pipe.read_to_end(&mut response))
    .await
    .context("timeout reading named pipe response")?
    .context("read named pipe response failed")?;

  decode_http_response_bytes(&response)
}

#[cfg(windows)]
fn non_empty(v: &str) -> Option<&str> {
  let t = v.trim();
  if t.is_empty() { None } else { Some(t) }
}

#[cfg(not(windows))]
pub(crate) async fn request_via_npipe<T: DeserializeOwned>(
  _pipe_name: &str,
  _method: &str,
  _path: &str,
  _body: Option<Vec<u8>>,
  _token: Option<&str>,
  _timeout_value: Duration,
) -> anyhow::Result<T> {
  bail!("npipe endpoint is not supported on this platform target")
}

#[cfg(windows)]
fn decode_http_response_bytes<T: DeserializeOwned>(raw: &[u8]) -> anyhow::Result<T> {
  const HEADER_END: &[u8] = b"\r\n\r\n";
  let split_pos = raw
    .windows(HEADER_END.len())
    .position(|window| window == HEADER_END)
    .ok_or_else(|| anyhow!("invalid http response from named pipe: header terminator not found"))?;

  let header_bytes = &raw[..split_pos];
  let body_bytes = &raw[(split_pos + HEADER_END.len())..];
  let header_text = String::from_utf8_lossy(header_bytes);
  let status_line = header_text
    .lines()
    .next()
    .ok_or_else(|| anyhow!("invalid http response from named pipe: empty status line"))?;
  let status = parse_status_code(status_line)?;

  let body_text = String::from_utf8_lossy(body_bytes).trim().to_string();
  if status >= 400 {
    bail!(
      "request failed via named pipe: status={} body={}",
      status,
      body_text
    );
  }

  let envelope: ApiEnvelope<T> =
    serde_json::from_slice(body_bytes).context("decode api envelope from named pipe response")?;
  if envelope.code != 0 {
    bail!(
      "api error: code={} message={}",
      envelope.code,
      envelope.message
    );
  }
  Ok(envelope.data)
}

#[cfg(windows)]
fn parse_status_code(status_line: &str) -> anyhow::Result<u16> {
  let mut parts = status_line.split_whitespace();
  let _http_version = parts
    .next()
    .ok_or_else(|| anyhow!("invalid http status line from named pipe"))?;
  let code = parts
    .next()
    .ok_or_else(|| anyhow!("missing status code in named pipe response"))?;
  code
    .parse::<u16>()
    .with_context(|| format!("parse status code from named pipe response: {code}"))
}
