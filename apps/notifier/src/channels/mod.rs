//! Channel implementations. Each channel is responsible for its own delivery
//! mechanics; retry/backoff/DLQ decisions live one level up in main.rs, which
//! calls these and interprets the Result.
pub mod email;
pub mod webhook;

/// Common outcome every channel reports back to the dispatch loop.
pub struct DeliveryResult {
    pub success: bool,
    pub status_code: Option<i32>,
    pub error: Option<String>,
    pub latency_ms: i64,
}
