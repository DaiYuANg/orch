use axum::{
  body::{Body, to_bytes},
  http::{Request, StatusCode},
};
use tower::ServiceExt;
use warden_api::{ApiState, router};
use warden_dns::DnsService;
use warden_registry::RegistryService;
use warden_runtime::RuntimeEngine;
use warden_store::StateStore;
use warden_task::TaskService;
use warden_types::{ApiEnvelope, api_code};

#[tokio::test]
async fn wraps_non_json_errors_with_api_envelope() {
  let app = build_router();
  let response = app
    .oneshot(
      Request::builder()
        .uri("/not-found")
        .body(Body::empty())
        .unwrap(),
    )
    .await
    .unwrap();

  assert_eq!(response.status(), StatusCode::NOT_FOUND);
  let body = to_bytes(response.into_body(), 1024 * 64).await.unwrap();
  let envelope: ApiEnvelope<String> = serde_json::from_slice(&body).unwrap();
  assert_eq!(envelope.code, api_code::NOT_FOUND);
}

#[tokio::test]
async fn preserves_json_error_from_handler() {
  let app = build_router();
  let response = app
    .oneshot(
      Request::builder()
        .uri("/tasks/not-exists")
        .body(Body::empty())
        .unwrap(),
    )
    .await
    .unwrap();

  assert_eq!(response.status(), StatusCode::NOT_FOUND);
  let body = to_bytes(response.into_body(), 1024 * 64).await.unwrap();
  let envelope: ApiEnvelope<String> = serde_json::from_slice(&body).unwrap();
  assert_eq!(envelope.code, api_code::NOT_FOUND);
  assert!(envelope.message.contains("not-exists"));
}

#[tokio::test]
async fn exposes_openapi_json_and_swagger_ui() {
  let app = build_router();

  let openapi = app
    .clone()
    .oneshot(
      Request::builder()
        .uri("/api-docs/openapi.json")
        .body(Body::empty())
        .unwrap(),
    )
    .await
    .unwrap();
  assert_eq!(openapi.status(), StatusCode::OK);

  let swagger = app
    .oneshot(
      Request::builder()
        .uri("/swagger-ui/")
        .body(Body::empty())
        .unwrap(),
    )
    .await
    .unwrap();
  assert_eq!(swagger.status(), StatusCode::OK);
}

fn build_router() -> axum::Router {
  let store = StateStore::new();
  let state = ApiState {
    registry: RegistryService::new(store.clone()),
    dns: DnsService::new(store.clone()),
    task: TaskService::new(RuntimeEngine::new(), store),
    raft_enabled: false,
    raft_node_id: 1,
    raft_bind_addr: String::from("127.0.0.1:12000"),
  };
  router(state)
}
