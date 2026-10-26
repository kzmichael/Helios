//! SMS delivery through an HTTP SMS gateway (a Twilio-style form POST).
//!
//! In every environment before real production, `SMS_GATEWAY_URL` must point at a
//! local sink, never a live provider: like email, no development code path may
//! reach a real handset. Without a gateway configured the channel is simply
//! unavailable and SMS requests are dead-lettered (see main.rs).
use std::time::Instant;

use super::DeliveryResult;

/// Longest message body sent; carriers split or reject beyond this.
const MAX_BODY_CHARS: usize = 1600;

pub struct SmsChannel {
    client: reqwest::Client,
    gateway_url: String,
    from: String,
    auth: Option<(String, String)>,
}

impl SmsChannel {
    pub fn new(
        gateway_url: &str,
        from: &str,
        auth: Option<(String, String)>,
        timeout_secs: u64,
    ) -> Self {
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(timeout_secs))
            .build()
            .expect("failed to build HTTP client");
        Self {
            client,
            gateway_url: gateway_url.to_string(),
            from: from.to_string(),
            auth,
        }
    }

    pub async fn send(&self, to: &str, body: &str) -> DeliveryResult {
        let started = Instant::now();
        let text: String = body.chars().take(MAX_BODY_CHARS).collect();

        let mut request = self
            .client
            .post(&self.gateway_url)
            .form(&[("To", to), ("From", self.from.as_str()), ("Body", text.as_str())]);
        if let Some((user, password)) = &self.auth {
            request = request.basic_auth(user, Some(password));
        }

        match request.send().await {
            Ok(resp) => {
                let status = resp.status().as_u16() as i32;
                let ok = resp.status().is_success();
                DeliveryResult {
                    success: ok,
                    status_code: Some(status),
                    error: if ok { None } else { Some(format!("non-2xx status: {status}")) },
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
