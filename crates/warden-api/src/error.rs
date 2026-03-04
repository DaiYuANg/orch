use crate::ApiErr;
use axum::{
  Json,
  extract::Request,
  http::{StatusCode, header::CONTENT_TYPE},
  middleware::Next,
  response::{IntoResponse, Response},
};
use warden_types::{ApiEnvelope, api_code};

pub(crate) fn bad_request(message: String) -> ApiErr {
  api_error(StatusCode::BAD_REQUEST, api_code::INVALID_ARGUMENT, message)
}

pub(crate) fn not_found(message: String) -> ApiErr {
  api_error(StatusCode::NOT_FOUND, api_code::NOT_FOUND, message)
}

pub(crate) fn internal_error(err: impl std::fmt::Display) -> ApiErr {
  api_error(
    StatusCode::INTERNAL_SERVER_ERROR,
    api_code::INTERNAL,
    err.to_string(),
  )
}

pub(crate) async fn error_envelope_middleware(req: Request, next: Next) -> Response {
  let response = next.run(req).await;
  let status = response.status();
  if !status.is_client_error() && !status.is_server_error() {
    return response;
  }

  let content_type = response
    .headers()
    .get(CONTENT_TYPE)
    .and_then(|v| v.to_str().ok())
    .unwrap_or_default()
    .to_ascii_lowercase();
  if content_type.contains("application/json") {
    return response;
  }

  (
    status,
    Json(ApiEnvelope::err(
      status_to_api_code(status),
      status
        .canonical_reason()
        .unwrap_or("request failed")
        .to_string(),
    )),
  )
    .into_response()
}

pub(crate) fn status_to_api_code(status: StatusCode) -> i32 {
  match status {
    StatusCode::BAD_REQUEST => api_code::INVALID_ARGUMENT,
    StatusCode::NOT_FOUND => api_code::NOT_FOUND,
    _ => api_code::INTERNAL,
  }
}

fn api_error(status: StatusCode, code: i32, message: String) -> ApiErr {
  (status, Json(ApiEnvelope::err(code, message)))
}
