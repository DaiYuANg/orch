use axum::body::{Body, to_bytes};
use axum::http::header::HOST;
use axum::http::{Request, Response, StatusCode};

pub async fn proxy_http_request(
  client: &reqwest::Client,
  max_request_body: usize,
  req: Request<Body>,
  backend: String,
) -> Response<Body> {
  let target = if let Some(value) = backend.strip_prefix("http://") {
    value.to_string()
  } else if backend.starts_with("https://") {
    return response_with_status(
      StatusCode::BAD_GATEWAY,
      "https backend is not supported yet",
    );
  } else {
    backend
  };

  let path_and_query = req
    .uri()
    .path_and_query()
    .map(|v| v.as_str().to_string())
    .unwrap_or_else(|| String::from("/"));
  let target_url = format!("http://{target}{path_and_query}");
  let (parts, body) = req.into_parts();
  let body_bytes = match to_bytes(body, max_request_body).await {
    Ok(payload) => payload,
    Err(err) => {
      return response_with_status(
        StatusCode::BAD_GATEWAY,
        &format!("proxy read body error: {err}"),
      );
    }
  };

  let mut outbound = client
    .request(parts.method.clone(), target_url)
    .body(body_bytes.to_vec());
  for (name, value) in &parts.headers {
    if *name != HOST {
      outbound = outbound.header(name, value);
    }
  }

  let upstream = match outbound.send().await {
    Ok(resp) => resp,
    Err(err) => {
      return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
    }
  };
  let status = upstream.status();
  let headers = upstream.headers().clone();
  let payload = match upstream.bytes().await {
    Ok(data) => data,
    Err(err) => {
      return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
    }
  };

  let mut response = Response::new(Body::from(payload));
  *response.status_mut() = status;
  for (name, value) in &headers {
    response.headers_mut().insert(name, value.clone());
  }
  response
}

pub fn response_with_status(status: StatusCode, body: &str) -> Response<Body> {
  let mut response = Response::new(Body::from(body.to_string()));
  *response.status_mut() = status;
  response
}
