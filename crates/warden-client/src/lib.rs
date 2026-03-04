use anyhow::{Context, anyhow, bail};
use serde::{Serialize, de::DeserializeOwned};
use std::path::PathBuf;
use std::time::Duration;
use warden_types::ApiEnvelope;

#[derive(Debug, Clone)]
pub enum Endpoint {
    Http(String),
    Unix(PathBuf),
    Npipe(String),
    Auto,
}

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
        self.request_once("GET", &self.endpoint, &uri_path, None)
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
        self.request_once("POST", &self.endpoint, &uri_path, Some(payload))
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
        match endpoint {
            Endpoint::Http(base) => {
                let url = format!("{}{}", base.trim_end_matches('/'), path);
                let client = reqwest::Client::builder().timeout(self.timeout).build()?;
                let req = client.request(parse_http_method(method)?, url);
                let req = if let Some(payload) = body {
                    req.header("Content-Type", "application/json").body(payload)
                } else {
                    req
                };
                let req = attach_auth(req, self.token.as_deref());
                decode_response(req.send().await?).await
            }
            Endpoint::Unix(socket) => {
                #[cfg(unix)]
                {
                    let client = reqwest::Client::builder()
                        .unix_socket(socket)
                        .timeout(self.timeout)
                        .build()
                        .context("build unix socket client")?;
                    let req = client.request(
                        parse_http_method(method)?,
                        format!("http://localhost{}", path),
                    );
                    let req = if let Some(payload) = body {
                        req.header("Content-Type", "application/json").body(payload)
                    } else {
                        req
                    };
                    let req = attach_auth(req, self.token.as_deref());
                    decode_response(req.send().await?).await
                }
                #[cfg(not(unix))]
                {
                    let _ = socket;
                    bail!("unix socket endpoint is not supported on this platform target");
                }
            }
            Endpoint::Npipe(pipe_name) => {
                #[cfg(windows)]
                {
                    self.request_via_npipe(pipe_name, method, path, body).await
                }
                #[cfg(not(windows))]
                {
                    let _ = pipe_name;
                    bail!("npipe endpoint is not supported on this platform target")
                }
            }
            Endpoint::Auto => {
                bail!("auto endpoint should be resolved before single-attempt request")
            }
        }
    }
}

pub fn parse_endpoint(raw: &str) -> anyhow::Result<Endpoint> {
    let trimmed = raw.trim();
    if trimmed.is_empty() || trimmed.eq_ignore_ascii_case("auto") {
        return Ok(Endpoint::Auto);
    }
    if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        return Ok(Endpoint::Http(trimmed.trim_end_matches('/').to_string()));
    }
    if let Some(rest) = trimmed.strip_prefix("unix://") {
        let path = rest.trim();
        if path.is_empty() {
            bail!("invalid unix endpoint: empty socket path");
        }
        return Ok(Endpoint::Unix(PathBuf::from(path)));
    }
    if let Some(rest) = trimmed.strip_prefix("npipe://") {
        let pipe = normalize_npipe_path(rest);
        if pipe.is_empty() {
            bail!("invalid npipe endpoint: empty pipe path");
        }
        return Ok(Endpoint::Npipe(pipe));
    }
    Err(anyhow!(
        "unsupported endpoint {trimmed}, expected auto|unix://|npipe://|http(s)://"
    ))
}

fn auto_endpoints() -> Vec<Endpoint> {
    #[cfg(windows)]
    {
        vec![
            Endpoint::Npipe(String::from(r"\\.\pipe\warden")),
            Endpoint::Http(String::from("http://127.0.0.1:7443")),
        ]
    }

    #[cfg(not(windows))]
    {
        let socket = std::env::temp_dir().join("warden.sock");
        vec![
            Endpoint::Unix(socket),
            Endpoint::Http(String::from("http://127.0.0.1:7443")),
        ]
    }
}

fn normalize_npipe_path(raw: &str) -> String {
    let mut value = raw.trim().replace('/', "\\");
    if value.is_empty() {
        return value;
    }
    if value.starts_with(r"\\.\pipe\") {
        return value;
    }
    value = value.trim_start_matches('\\').to_string();
    if let Some(rest) = value.strip_prefix("pipe\\") {
        return format!(r"\\.\pipe\{}", rest.trim_start_matches('\\'));
    }
    if let Some(rest) = value.strip_prefix(".\\pipe\\") {
        return format!(r"\\.\pipe\{}", rest.trim_start_matches('\\'));
    }
    format!(r"\\.\pipe\{}", value)
}

fn describe_endpoint(endpoint: &Endpoint) -> String {
    match endpoint {
        Endpoint::Http(base) => format!("http({base})"),
        Endpoint::Unix(path) => format!("unix({})", path.display()),
        Endpoint::Npipe(path) => format!("npipe({path})"),
        Endpoint::Auto => String::from("auto"),
    }
}

fn ensure_path(path: &str) -> String {
    let p = path.trim();
    if p.is_empty() {
        return String::from("/");
    }
    if p.starts_with('/') {
        p.to_string()
    } else {
        format!("/{p}")
    }
}

fn attach_auth(req: reqwest::RequestBuilder, token: Option<&str>) -> reqwest::RequestBuilder {
    if let Some(value) = token.and_then(non_empty) {
        req.bearer_auth(value)
    } else {
        req
    }
}

fn parse_http_method(method: &str) -> anyhow::Result<reqwest::Method> {
    reqwest::Method::from_bytes(method.as_bytes())
        .with_context(|| format!("invalid http method: {method}"))
}

fn non_empty(v: &str) -> Option<&str> {
    let t = v.trim();
    if t.is_empty() { None } else { Some(t) }
}

async fn decode_response<T: DeserializeOwned>(resp: reqwest::Response) -> anyhow::Result<T> {
    let status = resp.status();
    let body = resp.text().await.context("read response body")?;

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

#[cfg(windows)]
impl WardenClient {
    async fn request_via_npipe<T: DeserializeOwned>(
        &self,
        pipe_name: &str,
        method: &str,
        path: &str,
        body: Option<Vec<u8>>,
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
        if let Some(token) = self.token.as_deref().and_then(non_empty) {
            request.push_str("Authorization: Bearer ");
            request.push_str(token);
            request.push_str("\r\n");
        }
        request.push_str("\r\n");

        timeout(self.timeout, pipe.write_all(request.as_bytes()))
            .await
            .context("timeout writing named pipe request")?
            .context("write named pipe request failed")?;
        if !payload.is_empty() {
            timeout(self.timeout, pipe.write_all(&payload))
                .await
                .context("timeout writing named pipe body")?
                .context("write named pipe body failed")?;
        }
        timeout(self.timeout, pipe.flush())
            .await
            .context("timeout flushing named pipe request")?
            .context("flush named pipe request failed")?;

        let mut response = Vec::with_capacity(4096);
        timeout(self.timeout, pipe.read_to_end(&mut response))
            .await
            .context("timeout reading named pipe response")?
            .context("read named pipe response failed")?;

        decode_http_response_bytes(&response)
    }
}

#[cfg(windows)]
fn decode_http_response_bytes<T: DeserializeOwned>(raw: &[u8]) -> anyhow::Result<T> {
    const HEADER_END: &[u8] = b"\r\n\r\n";
    let split_pos = raw
        .windows(HEADER_END.len())
        .position(|window| window == HEADER_END)
        .ok_or_else(|| {
            anyhow!("invalid http response from named pipe: header terminator not found")
        })?;

    let header_bytes = &raw[..split_pos];
    let body_bytes = &raw[(split_pos + HEADER_END.len())..];

    let header_text = String::from_utf8_lossy(header_bytes);
    let mut lines = header_text.lines();
    let status_line = lines
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

    let envelope: ApiEnvelope<T> = serde_json::from_slice(body_bytes)
        .context("decode api envelope from named pipe response")?;
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
    code.parse::<u16>()
        .with_context(|| format!("parse status code from named pipe response: {code}"))
}
