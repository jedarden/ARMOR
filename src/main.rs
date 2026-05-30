use actix_web::{get, App, HttpServer, HttpResponse, Responder};
use std::env;

#[get("/")]
async fn index() -> impl Responder {
    HttpResponse::Ok().json(serde_json::json!({
        "message": "ARMOR server is running",
        "status": "healthy"
    }))
}

#[get("/health")]
async fn health() -> impl Responder {
    HttpResponse::Ok().json(serde_json::json!({
        "status": "ok"
    }))
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    // Initialize logger
    env_logger::init_from_env(env_logger::Env::new().default_filter_or("info"));

    // Get port from environment variable or use default
    let port = env::var("ARMOR_PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(8080);

    let bind_address = format!("0.0.0.0:{}", port);

    log::info!("Starting ARMOR server on {}", bind_address);

    HttpServer::new(|| {
        App::new()
            .service(index)
            .service(health)
    })
    .bind(&bind_address)?
    .run()
    .await
}
