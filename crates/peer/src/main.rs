mod http;
mod config;

use crate::http::ApiDoc;
use axum::{
  http::StatusCode, routing::{get, post},
  Json,
  Router,
};
use axum_prometheus::PrometheusMetricLayer;
use dotenvy::dotenv;
use nject::provider;
use serde::{Deserialize, Serialize};
use tower_http::trace::TraceLayer;
use tracing::info;
use tracing_subscriber::EnvFilter;
use utoipa::OpenApi;
use utoipa_swagger_ui::SwaggerUi;

fn init() {
  dotenv().ok();
  tracing_subscriber::fmt()
    .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
    .with_env_filter(EnvFilter::from_default_env().add_directive("debug".parse().unwrap()))
    .with_ansi(true)
    .init();
}
#[provider]
struct Provider;
#[tokio::main]
async fn main() {
  init();
  let (prom_layer, metric_handle) = PrometheusMetricLayer::pair();
  // build our application with a route
  let app = Router::new()
    // `GET /` goes to `root`
    .route("/", get(root))
    // `POST /users` goes to `create_user`
    .route("/users", post(create_user))
    .route(
      "/metrics",
      get(move || async move { metric_handle.render() }),
    )
    .merge(SwaggerUi::new("/swagger").url("/api-doc/openapi.json", ApiDoc::openapi()))
    .layer(prom_layer)
    .layer(TraceLayer::new_for_http());

  // run our app with hyper, listening globally on port 3000
  let listener = tokio::net::TcpListener::bind("0.0.0.0:3000").await.unwrap();
  info!("Server start at http://{}", "localhost:3000");
  axum::serve(listener, app).await.unwrap();
}

// basic handler that responds with a static string
async fn root() -> &'static str {
  "Hello, World!"
}

async fn create_user(
  // this argument tells axum to parse the request body
  // as JSON into a `CreateUser` type
  Json(payload): Json<CreateUser>,
) -> (StatusCode, Json<User>) {
  // insert your application logic here
  let user = User {
    id: 1337,
    username: payload.username,
  };

  // this will be converted into a JSON response
  // with a status code of `201 Created`
  (StatusCode::CREATED, Json(user))
}

// the input to our `create_user` handler
#[derive(Deserialize)]
struct CreateUser {
  username: String,
}

// the output to our `create_user` handler
#[derive(Serialize)]
struct User {
  id: u64,
  username: String,
}
