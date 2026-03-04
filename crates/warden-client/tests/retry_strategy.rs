use axum::{
  Json, Router,
  extract::State,
  http::StatusCode,
  routing::{get, post},
};
use std::sync::{
  Arc,
  atomic::{AtomicUsize, Ordering},
};
use std::time::Duration;
use warden_client::{Endpoint, WardenClient};
use warden_types::{ApiEnvelope, api_code};

#[derive(Clone)]
struct RetryState {
  get_hits: Arc<AtomicUsize>,
  post_hits: Arc<AtomicUsize>,
}

impl RetryState {
  fn new() -> Self {
    Self {
      get_hits: Arc::new(AtomicUsize::new(0)),
      post_hits: Arc::new(AtomicUsize::new(0)),
    }
  }
}

#[tokio::test]
async fn retries_get_on_transient_error() -> anyhow::Result<()> {
  let state = RetryState::new();
  let (base, handle) = spawn_server(state.clone()).await?;
  let client = WardenClient::new(Endpoint::Http(base), None, Duration::from_secs(2));

  let value: String = client.get("/retry-get").await?;
  assert_eq!(value, "ok");
  assert_eq!(state.get_hits.load(Ordering::SeqCst), 2);

  handle.abort();
  Ok(())
}

#[tokio::test]
async fn does_not_retry_post_on_transient_error() -> anyhow::Result<()> {
  let state = RetryState::new();
  let (base, handle) = spawn_server(state.clone()).await?;
  let client = WardenClient::new(Endpoint::Http(base), None, Duration::from_secs(2));

  let payload = serde_json::json!({ "name": "demo" });
  let result: anyhow::Result<String> = client.post("/retry-post", &payload).await;
  assert!(result.is_err());
  assert_eq!(state.post_hits.load(Ordering::SeqCst), 1);

  handle.abort();
  Ok(())
}

async fn spawn_server(state: RetryState) -> anyhow::Result<(String, tokio::task::JoinHandle<()>)> {
  let app = Router::new()
    .route("/retry-get", get(flaky_get))
    .route("/retry-post", post(flaky_post))
    .with_state(state);
  let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await?;
  let addr = listener.local_addr()?;
  let handle = tokio::spawn(async move {
    let _ = axum::serve(listener, app).await;
  });
  Ok((format!("http://{addr}"), handle))
}

async fn flaky_get(State(state): State<RetryState>) -> (StatusCode, Json<ApiEnvelope<String>>) {
  let hits = state.get_hits.fetch_add(1, Ordering::SeqCst);
  if hits == 0 {
    return (
      StatusCode::INTERNAL_SERVER_ERROR,
      Json(ApiEnvelope::err(api_code::INTERNAL, "temporary error")),
    );
  }
  (StatusCode::OK, Json(ApiEnvelope::ok(String::from("ok"))))
}

async fn flaky_post(State(state): State<RetryState>) -> (StatusCode, Json<ApiEnvelope<String>>) {
  let hits = state.post_hits.fetch_add(1, Ordering::SeqCst);
  if hits == 0 {
    return (
      StatusCode::INTERNAL_SERVER_ERROR,
      Json(ApiEnvelope::err(api_code::INTERNAL, "temporary error")),
    );
  }
  (
    StatusCode::OK,
    Json(ApiEnvelope::ok(String::from("created"))),
  )
}
