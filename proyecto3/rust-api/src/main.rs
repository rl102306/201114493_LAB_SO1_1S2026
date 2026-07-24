use axum::{
    extract::State,
    http::StatusCode,
    routing::post,
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::env;
use reqwest::Client;
use tracing::info;

#[derive(Debug, Deserialize, Serialize, Clone)]
struct WarReport {
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    timestamp: String,
}

#[derive(Clone)]
struct AppState {
    http_client: Client,
    grpc_client_url: String,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let grpc_client_url = env::var("GRPC_CLIENT_URL")
        .unwrap_or_else(|_| "http://go-grpc-client-svc:8080".to_string());

    let state = AppState {
        http_client: Client::builder()
            .timeout(std::time::Duration::from_secs(10))
            .build()
            .unwrap(),
        grpc_client_url,
    };

    let app = Router::new()
        .route("/grpc-201114493", post(handle_report))
        .with_state(state);

    let port = env::var("PORT").unwrap_or_else(|_| "8080".to_string());
    let addr = format!("0.0.0.0:{}", port);

    info!("rust-api listening on {}", addr);
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn handle_report(
    State(state): State<AppState>,
    Json(payload): Json<WarReport>,
) -> Result<Json<serde_json::Value>, (StatusCode, Json<serde_json::Value>)> {
    let valid_countries = ["USA", "RUS", "CHN", "ESP", "GTM"];
    if !valid_countries.contains(&payload.country.as_str()) {
        return Err((
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({"error": "Invalid country. Valid: USA, RUS, CHN, ESP, GTM"})),
        ));
    }

    if payload.warplanes_in_air < 0 || payload.warplanes_in_air > 50 {
        return Err((
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({"error": "warplanes_in_air must be 0-50"})),
        ));
    }

    if payload.warships_in_water < 0 || payload.warships_in_water > 30 {
        return Err((
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({"error": "warships_in_water must be 0-30"})),
        ));
    }

    info!("Received report: country={} warplanes={} warships={}", payload.country, payload.warplanes_in_air, payload.warships_in_water);

    let url = format!("{}/report", state.grpc_client_url);
    let resp = state
        .http_client
        .post(&url)
        .json(&payload)
        .send()
        .await
        .map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({"error": e.to_string()})),
            )
        })?;

    if resp.status().is_success() {
        Ok(Json(
            serde_json::json!({"status": "ok", "country": payload.country}),
        ))
    } else {
        Err((
            StatusCode::BAD_GATEWAY,
            Json(serde_json::json!({"error": "Failed to forward to gRPC client"})),
        ))
    }
}
