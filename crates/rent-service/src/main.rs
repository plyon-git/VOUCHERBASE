// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
use axum::{
    extract::{DefaultBodyLimit, State},
    http::{HeaderMap, StatusCode},
    routing::{get, post},
    Json, Router,
};
use rent_core::{calculate, Request};
use serde_json::{json, Value};
use std::{env, sync::Arc};
async fn compute(
    State(token): State<Arc<String>>,
    headers: HeaderMap,
    Json(input): Json<Request>,
) -> (StatusCode, Json<Value>) {
    let got = headers
        .get("x-internal-token")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    let mismatch = got.len() != token.len()
        || got
            .bytes()
            .zip(token.bytes())
            .fold(0u8, |a, (b, c)| a | (b ^ c))
            != 0;
    if mismatch {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({"error":"unauthorized"})),
        );
    }
    match calculate(&input) {
        Ok(out) => (StatusCode::OK, Json(serde_json::to_value(out).unwrap())),
        Err(e) => (StatusCode::UNPROCESSABLE_ENTITY, Json(json!({"error":e}))),
    }
}
#[tokio::main]
async fn main() {
    let token = env::var("VB_INTERNAL_TOKEN").expect("VB_INTERNAL_TOKEN is required");
    assert!(
        token.len() >= 32,
        "Internal token must be at least 32 characters"
    );
    let app = Router::new()
        .route(
            "/healthz",
            get(|| async { Json(json!({"status":"ok","engine":rent_core::VERSION})) }),
        )
        .route("/v1/calculate", post(compute))
        .layer(DefaultBodyLimit::max(1_048_576))
        .with_state(Arc::new(token));
    let listener = tokio::net::TcpListener::bind("0.0.0.0:8081").await.unwrap();
    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await
        .unwrap();
}
