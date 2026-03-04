use axum::{
  body::{Body, to_bytes},
  http::{Request, StatusCode},
};
use tower::ServiceExt;
use warden_api::{ApiState, router};
use warden_dns::DnsService;
use warden_raft::RaftService;
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

#[tokio::test]
async fn rejects_invalid_dsl_manifest() {
  let app = build_router();
  let payload = serde_json::json!({
    "manifest_yaml": "apiVersion: warden.io/v1alpha1\nkind: Application\nmetadata:\n  name: demo\nspec:\n  workloads: []\n",
    "prune": false,
    "strict": false,
    "concurrency": 4
  });
  let response = app
    .oneshot(
      Request::builder()
        .method("POST")
        .uri("/dsl/apply")
        .header("content-type", "application/json")
        .body(Body::from(payload.to_string()))
        .unwrap(),
    )
    .await
    .unwrap();

  assert_eq!(response.status(), StatusCode::BAD_REQUEST);
  let body = to_bytes(response.into_body(), 1024 * 64).await.unwrap();
  let envelope: ApiEnvelope<String> = serde_json::from_slice(&body).unwrap();
  assert_eq!(envelope.code, api_code::INVALID_ARGUMENT);
}

#[tokio::test]
async fn returns_not_found_for_missing_task_logs() {
  let app = build_router();
  let response = app
    .oneshot(
      Request::builder()
        .uri("/tasks/not-exists/logs?tail=20")
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

fn build_router() -> axum::Router {
  let store = StateStore::new();
  let state = ApiState {
    registry: RegistryService::new(store.clone()),
    dns: DnsService::new(store.clone()),
    task: TaskService::new(RuntimeEngine::new(), store),
    raft: RaftService::new(false, 1, String::from("127.0.0.1:12000")).unwrap(),
  };
  router(state)
}
