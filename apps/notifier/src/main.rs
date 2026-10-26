//! notifier: consumes trackline.notifications.requested and delivers via
//! email/webhook/sms. See apps/_contracts/kafka-topics.md for topic/consumer
//! group details and apps/_contracts/auth.md §7 for webhook signing.
mod channels;
mod dlq;
mod proto;
mod retry;

use rdkafka::config::ClientConfig;
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::message::Message;
use rdkafka::producer::{FutureProducer, FutureRecord};
use serde::Deserialize;
use std::collections::HashSet;
use std::env;
use std::sync::Mutex;
use std::time::Duration;

use channels::email::EmailChannel;
use channels::sms::SmsChannel;
use channels::webhook::WebhookChannel;

const REQUESTED_TOPIC: &str = "trackline.notifications.requested";
const DELIVERED_TOPIC: &str = "trackline.notifications.delivered";
const DEFAULT_CONSUMER_GROUP: &str = "notification-dispatcher";

/// A notification to deliver. Decoded from protobuf (`NotificationRequested` in
/// apps/_proto/trackline/v1/events.proto), or from JSON for hand-made test requests.
#[derive(Debug, Deserialize)]
pub struct NotificationRequested {
    #[serde(rename = "alertId")]
    pub alert_id: String,
    #[serde(rename = "idempotencyKey")]
    pub idempotency_key: String,
    #[serde(rename = "channelKind")]
    pub channel_kind: String, // "email" | "webhook" | "sms"
    pub recipient: String,
    pub subject: String,
    pub body: String,
    pub attempt: u32,
    #[serde(rename = "tenantId", default)]
    pub tenant_id: String,
    #[serde(rename = "channelId", default)]
    pub channel_id: String,
}

/// The final result of delivering one notification, reported on `notifications.delivered`.
pub struct Outcome {
    pub success: bool,
    pub attempt: u32,
    pub status_code: Option<i32>,
    pub error: Option<String>,
    pub latency_ms: i64,
}

/// Everything a delivery needs besides the notification itself.
struct Channels {
    email: EmailChannel,
    webhook: WebhookChannel,
    webhook_secret: Option<String>,
    sms: Option<SmsChannel>,
}

/// In-memory idempotency guard for Phase 0. Durable, shared dedup (Redis-backed)
/// is a Phase 2 concern — see the equivalent note in apps/ingest/README.md.
struct SeenKeys(Mutex<HashSet<String>>);

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let brokers = env::var("KAFKA_BROKERS").unwrap_or_else(|_| "kafka:9092".into());
    let group_id = env::var("KAFKA_GROUP_ID").unwrap_or_else(|_| DEFAULT_CONSUMER_GROUP.into());
    let smtp_host = env::var("SMTP_HOST").unwrap_or_else(|_| "mailpit".into());
    let smtp_port: u16 = env::var("SMTP_PORT").ok().and_then(|s| s.parse().ok()).unwrap_or(1025);
    let from_addr = env::var("SMTP_FROM").unwrap_or_else(|_| "alerts@trackline.local".into());
    let webhook_timeout: u64 = env::var("WEBHOOK_TIMEOUT_SECS").ok().and_then(|s| s.parse().ok()).unwrap_or(5);
    let webhook_secret = env::var("WEBHOOK_SECRET").ok().filter(|s| !s.is_empty());
    let sms_gateway = env::var("SMS_GATEWAY_URL").ok().filter(|s| !s.is_empty());

    if webhook_secret.is_none() {
        tracing::warn!("WEBHOOK_SECRET is not set; webhook notifications will be dead-lettered");
    }
    if sms_gateway.is_none() {
        tracing::warn!("SMS_GATEWAY_URL is not set; SMS notifications will be dead-lettered");
    }

    let consumer: StreamConsumer = ClientConfig::new()
        .set("bootstrap.servers", &brokers)
        .set("group.id", &group_id)
        .set("auto.offset.reset", "earliest")
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
        .expect("failed to create Kafka producer (for DLQ and delivered events)");

    let channels = Channels {
        email: EmailChannel::new(&smtp_host, smtp_port, &from_addr).expect("failed to build email transport"),
        webhook: WebhookChannel::new(webhook_timeout),
        webhook_secret,
        sms: sms_gateway.map(|url| {
            let auth = match (env::var("SMS_GATEWAY_USER"), env::var("SMS_GATEWAY_PASSWORD")) {
                (Ok(u), Ok(p)) if !u.is_empty() => Some((u, p)),
                _ => None,
            };
            SmsChannel::new(&url, &env::var("SMS_FROM").unwrap_or_else(|_| "TRACKLINE".into()), auth, webhook_timeout)
        }),
    };

    let seen = SeenKeys(Mutex::new(HashSet::new()));

    tracing::info!(brokers = %brokers, group = %group_id, "notifier started, consuming {REQUESTED_TOPIC}");

    let mut terminate = tokio::signal::unix::signal(
        tokio::signal::unix::SignalKind::terminate(),
    )
    .expect("failed to register SIGTERM handler");

    loop {
        let received = tokio::select! {
            _ = terminate.recv() => {
                tracing::info!("SIGTERM received; stopping notifier");
                break;
            }
            result = consumer.recv() => result,
        };

        match received {
            Err(e) => tracing::error!(error = %e, "kafka receive error"),
            Ok(msg) => {
                let payload = match msg.payload() {
                    Some(p) => p,
                    None => {
                        consumer.commit_message(&msg, rdkafka::consumer::CommitMode::Async).ok();
                        continue;
                    }
                };

                let parsed = proto::parse_requested(payload);
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

                        let outcome = deliver_with_retry(&notification, &channels, &producer).await;
                        report_outcome(&producer, &notification, &outcome).await;
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
    channels: &Channels,
    producer: &FutureProducer,
) -> Outcome {
    let mut attempt = n.attempt.max(1);

    loop {
        let result = match n.channel_kind.as_str() {
            "email" => channels.email.send(&n.recipient, &n.subject, &n.body).await,
            "webhook" => match &channels.webhook_secret {
                Some(secret) => channels.webhook.send(&n.recipient, secret, &n.body).await,
                None => return give_up(producer, n, attempt, "webhook_secret_not_configured").await,
            },
            "sms" => match &channels.sms {
                Some(sms) => sms.send(&n.recipient, &format!("{}\n{}", n.subject, n.body)).await,
                None => return give_up(producer, n, attempt, "sms_gateway_not_configured").await,
            },
            other => {
                tracing::error!(channel_kind = other, "unsupported channel, sending to DLQ");
                return give_up(producer, n, attempt, "unsupported_channel").await;
            }
        };

        if result.success {
            tracing::info!(alert_id = %n.alert_id, attempt, latency_ms = result.latency_ms, "delivered");
            return Outcome {
                success: true,
                attempt,
                status_code: result.status_code,
                error: None,
                latency_ms: result.latency_ms,
            };
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
            return Outcome {
                success: false,
                attempt,
                status_code: result.status_code,
                error: result.error,
                latency_ms: result.latency_ms,
            };
        }

        tokio::time::sleep(retry::backoff_for_attempt(attempt)).await;
        attempt += 1;
    }
}

/// A permanent failure: dead-letter immediately, no retries (kafka-topics.md §5).
async fn give_up(producer: &FutureProducer, n: &NotificationRequested, attempt: u32, reason: &str) -> Outcome {
    let _ = dlq::send_to_dlq(
        producer,
        n.body.as_bytes(),
        dlq::DlqReason { alert_id: &n.alert_id, reason, attempts: attempt },
    ).await;
    Outcome {
        success: false,
        // Past the retry budget on purpose: this is final, not a failure to retry.
        attempt: attempt.max(retry::MAX_ATTEMPTS),
        status_code: None,
        error: Some(reason.to_string()),
        latency_ms: 0,
    }
}

/// Publishes the final outcome so the platform can record what was actually sent.
async fn report_outcome(producer: &FutureProducer, n: &NotificationRequested, outcome: &Outcome) {
    let payload = proto::encode_delivered(n, outcome);
    let record = FutureRecord::to(DELIVERED_TOPIC).key(&n.alert_id).payload(&payload);
    if let Err((e, _)) = producer.send(record, Duration::from_secs(5)).await {
        tracing::error!(error = %e, alert_id = %n.alert_id, "failed to publish delivery outcome");
    }
}
