//! notifier: consumes trackline.notifications.requested and delivers via
//! email/webhook/sms. See apps/_contracts/kafka-topics.md for topic/consumer
//! group details and apps/_contracts/auth.md §7 for webhook signing.
mod channels;
mod dlq;
mod retry;

use rdkafka::config::ClientConfig;
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::message::Message;
use rdkafka::producer::FutureProducer;
use serde::Deserialize;
use std::collections::HashSet;
use std::env;
use std::sync::Mutex;

use channels::email::EmailChannel;
use channels::webhook::WebhookChannel;

const REQUESTED_TOPIC: &str = "trackline.notifications.requested";
const CONSUMER_GROUP: &str = "notification-dispatcher";

/// Mirrors NotificationRequested from apps/_proto/trackline/v1/events.proto.
/// Phase 0 decodes JSON for simplicity; switching to protobuf (`prost`) is a
/// small, contained change once the generated Rust bindings exist.
#[derive(Debug, Deserialize)]
struct NotificationRequested {
    #[serde(rename = "alertId")]
    alert_id: String,
    #[serde(rename = "idempotencyKey")]
    idempotency_key: String,
    #[serde(rename = "channelKind")]
    channel_kind: String, // "email" | "webhook" | "sms"
    recipient: String,
    subject: String,
    body: String,
    attempt: u32,
}

/// In-memory idempotency guard for Phase 0. Durable, shared dedup (Redis-backed)
/// is a Phase 2 concern — see the equivalent note in apps/ingest/README.md.
struct SeenKeys(Mutex<HashSet<String>>);

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let brokers = env::var("KAFKA_BROKERS").unwrap_or_else(|_| "kafka:9092".into());
    let smtp_host = env::var("SMTP_HOST").unwrap_or_else(|_| "mailpit".into());
    let smtp_port: u16 = env::var("SMTP_PORT").ok().and_then(|s| s.parse().ok()).unwrap_or(1025);
    let from_addr = env::var("SMTP_FROM").unwrap_or_else(|_| "alerts@trackline.local".into());
    let webhook_timeout: u64 = env::var("WEBHOOK_TIMEOUT_SECS").ok().and_then(|s| s.parse().ok()).unwrap_or(5);

    let consumer: StreamConsumer = ClientConfig::new()
        .set("bootstrap.servers", &brokers)
        .set("group.id", CONSUMER_GROUP)
        // Manual commit — commit AFTER successful processing, never before.
        // Committing early is how at-least-once quietly becomes at-most-once
        // and messages get lost on a crash (kafka-topics.md §3).
        .set("enable.auto.commit", "false")
        .create()
        .expect("failed to create Kafka consumer");

    consumer
        .subscribe(&[REQUESTED_TOPIC])
        .expect("failed to subscribe to notifications.requested");

    let producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &brokers)
        .create()
        .expect("failed to create Kafka producer (for DLQ)");

    let email = EmailChannel::new(&smtp_host, smtp_port, &from_addr)
        .expect("failed to build email transport");
    let webhook = WebhookChannel::new(webhook_timeout);

    let seen = SeenKeys(Mutex::new(HashSet::new()));

    tracing::info!(brokers = %brokers, "notifier started, consuming {REQUESTED_TOPIC}");

    loop {
        match consumer.recv().await {
            Err(e) => tracing::error!(error = %e, "kafka receive error"),
            Ok(msg) => {
                let payload = match msg.payload() {
                    Some(p) => p,
                    None => {
                        consumer.commit_message(&msg, rdkafka::consumer::CommitMode::Async).ok();
                        continue;
                    }
                };

                let parsed: Result<NotificationRequested, _> = serde_json::from_slice(payload);
                match parsed {
                    Err(e) => {
                        // Permanent failure — schema mismatch. Straight to DLQ,
                        // zero retries (kafka-topics.md §5).
                        tracing::error!(error = %e, "malformed notification, sending to DLQ");
                        let _ = dlq::send_to_dlq(
                            &producer,
                            payload,
                            dlq::DlqReason { alert_id: "unknown", reason: "schema_mismatch", attempts: 0 },
                        ).await;
                    }
                    Ok(notification) => {
                        {
                            let mut guard = seen.0.lock().unwrap();
                            if !guard.insert(notification.idempotency_key.clone()) {
                                tracing::info!(key = %notification.idempotency_key, "duplicate notification, skipping");
                                consumer.commit_message(&msg, rdkafka::consumer::CommitMode::Async).ok();
                                continue;
                            }
                        }

                        deliver_with_retry(&notification, &email, &webhook, &producer).await;
                    }
                }

                if let Err(e) = consumer.commit_message(&msg, rdkafka::consumer::CommitMode::Async) {
                    tracing::error!(error = %e, "failed to commit offset");
                }
            }
        }
    }
}

/// Retries transient failures with jittered exponential backoff
/// (retry::backoff_for_attempt) up to retry::MAX_ATTEMPTS, then gives up to the
/// DLQ. Does not retry inline for long periods — a slow retry here would hold
/// up this consumer's partition, which is exactly what kafka-topics.md §5 warns
/// against for poison messages. A bounded number of quick retries is the
/// accepted compromise for Phase 0; moving retries to a separate delayed-retry
/// topic is a reasonable Phase 2 improvement.
async fn deliver_with_retry(
    n: &NotificationRequested,
    email: &EmailChannel,
    webhook: &WebhookChannel,
    producer: &FutureProducer,
) {
    let mut attempt = n.attempt.max(1);

    loop {
        let result = match n.channel_kind.as_str() {
            "email" => email.send(&n.recipient, &n.subject, &n.body).await,
            "webhook" => webhook.send(&n.recipient, "dev-webhook-secret", &n.body).await,
            other => {
                tracing::error!(channel_kind = other, "unsupported channel, sending to DLQ");
                let _ = dlq::send_to_dlq(
                    producer,
                    n.body.as_bytes(),
                    dlq::DlqReason { alert_id: &n.alert_id, reason: "unsupported_channel", attempts: attempt },
                ).await;
                return;
            }
        };

        if result.success {
            tracing::info!(alert_id = %n.alert_id, attempt, latency_ms = result.latency_ms, "delivered");
            return;
        }

        tracing::warn!(
            alert_id = %n.alert_id, attempt, error = ?result.error,
            "delivery failed"
        );

        if retry::should_give_up(attempt) {
            let _ = dlq::send_to_dlq(
                producer,
                n.body.as_bytes(),
                dlq::DlqReason { alert_id: &n.alert_id, reason: "max_attempts_exceeded", attempts: attempt },
            ).await;
            return;
        }

        tokio::time::sleep(retry::backoff_for_attempt(attempt)).await;
        attempt += 1;
    }
}
