//! Email delivery via SMTP.
//!
//! In every environment before real production, this points at a local Mailpit
//! sink (see apps/_contracts/kafka-topics.md and 04_ENVIRONMENTS.md §9). No
//! development code path may be configured to reach a real inbox — protects the
//! lab domain's sending reputation before it even exists.
use lettre::message::Message;
use lettre::transport::smtp::AsyncSmtpTransport;
use lettre::{AsyncTransport, Tokio1Executor};
use std::time::Instant;

use super::DeliveryResult;

pub struct EmailChannel {
    transport: AsyncSmtpTransport<Tokio1Executor>,
    from: String,
}

impl EmailChannel {
    /// `smtp_host` in Phase 0 is the Mailpit container's SMTP port, never a
    /// real provider.
    pub fn new(smtp_host: &str, smtp_port: u16, from: &str) -> Result<Self, String> {
        let transport = AsyncSmtpTransport::<Tokio1Executor>::builder_dangerous(smtp_host)
            .port(smtp_port)
            .build();
        Ok(Self { transport, from: from.to_string() })
    }

    pub async fn send(&self, to: &str, subject: &str, body: &str) -> DeliveryResult {
        let started = Instant::now();

        let message = match Message::builder()
            .from(self.from.parse().unwrap())
            .to(match to.parse() {
                Ok(addr) => addr,
                Err(e) => {
                    return DeliveryResult {
                        success: false,
                        status_code: None,
                        error: Some(format!("invalid recipient: {e}")),
                        latency_ms: started.elapsed().as_millis() as i64,
                    }
                }
            })
            .subject(subject)
            .body(body.to_string())
        {
            Ok(m) => m,
            Err(e) => {
                return DeliveryResult {
                    success: false,
                    status_code: None,
                    error: Some(format!("message build failed: {e}")),
                    latency_ms: started.elapsed().as_millis() as i64,
                }
            }
        };

        match self.transport.send(message).await {
            Ok(_) => DeliveryResult {
                success: true,
                status_code: Some(250),
                error: None,
                latency_ms: started.elapsed().as_millis() as i64,
            },
            Err(e) => DeliveryResult {
                success: false,
                status_code: None,
                error: Some(e.to_string()),
                latency_ms: started.elapsed().as_millis() as i64,
            },
        }
    }
}
