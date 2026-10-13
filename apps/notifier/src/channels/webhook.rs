//! Outbound webhook delivery with HMAC signing, per apps/_contracts/auth.md §7.
//!
//! Receivers verify `X-Trackline-Signature` and must reject timestamps outside
//! a ±5 minute window to prevent replay. This module produces exactly that
//! header; it does not implement the receiver side (that's the customer's job).
use hmac::{Hmac, Mac};
use sha2::Sha256;
use std::time::{Instant, SystemTime, UNIX_EPOCH};
use uuid::Uuid;

use super::DeliveryResult;

type HmacSha256 = Hmac<Sha256>;

pub struct WebhookChannel {
    client: reqwest::Client,
}

impl WebhookChannel {
    pub fn new(timeout_secs: u64) -> Self {
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(timeout_secs))
            .build()
            .expect("failed to build HTTP client");
        Self { client }
    }

    /// Signs `body` as `"{t}.{raw_body}"` keyed by `secret` — the exact scheme
    /// documented in auth.md §7, so a customer's receiver implementation can be
    /// verified against that doc alone, without reading this code.
    fn sign(secret: &str, timestamp: u64, body: &str) -> String {
        let mut mac = HmacSha256::new_from_slice(secret.as_bytes())
            .expect("HMAC accepts keys of any length");
        let signed_payload = format!("{timestamp}.{body}");
        mac.update(signed_payload.as_bytes());
        let result = hex::encode(mac.finalize().into_bytes());
        format!("t={timestamp},v1={result}")
    }

    pub async fn send(&self, url: &str, secret: &str, body: &str) -> DeliveryResult {
        let started = Instant::now();
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();

        let signature = Self::sign(secret, timestamp, body);
        let delivery_id = Uuid::new_v4().to_string();

        let result = self
            .client
            .post(url)
            .header("Content-Type", "application/json")
            .header("X-Trackline-Signature", signature)
            .header("X-Trackline-Delivery", delivery_id)
            .body(body.to_string())
            .send()
            .await;

        match result {
            Ok(resp) => {
                let status = resp.status().as_u16() as i32;
                DeliveryResult {
                    success: resp.status().is_success(),
                    status_code: Some(status),
                    error: if resp.status().is_success() {
                        None
                    } else {
                        Some(format!("non-2xx status: {status}"))
                    },
                    latency_ms: started.elapsed().as_millis() as i64,
                }
            }
            Err(e) => DeliveryResult {
                success: false,
                status_code: None,
                error: Some(e.to_string()),
                latency_ms: started.elapsed().as_millis() as i64,
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn signature_is_deterministic_for_same_inputs() {
        let a = WebhookChannel::sign("secret", 1000, "{}");
        let b = WebhookChannel::sign("secret", 1000, "{}");
        assert_eq!(a, b);
    }
}
