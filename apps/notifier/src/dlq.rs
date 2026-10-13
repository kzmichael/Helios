//! Dead-letter handling per apps/_contracts/kafka-topics.md §5.
//!
//! A message goes here after exhausting retry attempts, or immediately on a
//! permanent failure (schema mismatch, unknown tenant). The rule that matters:
//! **never block a partition retrying a poison message** — one unparseable
//! notification must not stall delivery for every other alert on the same
//! partition. This module is the escape hatch that keeps that true.
use rdkafka::producer::{FutureProducer, FutureRecord};
use std::time::Duration;

pub const NOTIFICATIONS_DLQ_TOPIC: &str = "trackline.notifications.dlq";

pub struct DlqReason<'a> {
    pub alert_id: &'a str,
    pub reason: &'a str,
    pub attempts: u32,
}

pub async fn send_to_dlq(
    producer: &FutureProducer,
    payload: &[u8],
    reason: DlqReason<'_>,
) -> Result<(), String> {
    tracing::warn!(
        alert_id = reason.alert_id,
        attempts = reason.attempts,
        reason = reason.reason,
        "sending notification to DLQ"
    );

    let record = FutureRecord::to(NOTIFICATIONS_DLQ_TOPIC)
        .key(reason.alert_id)
        .payload(payload);

    producer
        .send(record, Duration::from_secs(5))
        .await
        .map(|_| ())
        .map_err(|(err, _)| err.to_string())
}
