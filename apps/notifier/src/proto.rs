//! Wire formats for the notification topics.
//!
//! These structs mirror `NotificationRequested`, `NotificationDelivered` and
//! `EventMeta` in apps/_proto/trackline/v1/{events,common}.proto and must keep the
//! same field numbers. They are written by hand so the image build needs neither
//! `protoc` nor the generated bindings (which live outside this crate's build
//! context). Change the proto, change these.
use prost::Message;

#[derive(Clone, PartialEq, Message)]
pub struct EventMeta {
    #[prost(string, tag = "1")]
    pub event_id: String,
    #[prost(string, tag = "2")]
    pub tenant_id: String,
    #[prost(string, tag = "3")]
    pub source: String,
    #[prost(int64, tag = "4")]
    pub occurred_at: i64,
    #[prost(string, tag = "5")]
    pub traceparent: String,
    #[prost(string, tag = "6")]
    pub idempotency_key: String,
}

#[derive(Clone, PartialEq, Message)]
pub struct RequestedMsg {
    #[prost(message, optional, tag = "1")]
    pub meta: Option<EventMeta>,
    #[prost(string, tag = "2")]
    pub alert_id: String,
    #[prost(string, tag = "3")]
    pub channel_id: String,
    #[prost(string, tag = "4")]
    pub channel_kind: String,
    #[prost(string, tag = "5")]
    pub recipient: String,
    #[prost(string, tag = "6")]
    pub subject: String,
    #[prost(string, tag = "7")]
    pub body: String,
    #[prost(uint32, tag = "8")]
    pub attempt: u32,
}

#[derive(Clone, PartialEq, Message)]
pub struct DeliveredMsg {
    #[prost(message, optional, tag = "1")]
    pub meta: Option<EventMeta>,
    #[prost(string, tag = "2")]
    pub alert_id: String,
    #[prost(string, tag = "3")]
    pub channel_id: String,
    #[prost(bool, tag = "4")]
    pub success: bool,
    #[prost(uint32, tag = "5")]
    pub attempt: u32,
    #[prost(int32, tag = "6")]
    pub status_code: i32,
    #[prost(string, tag = "7")]
    pub error: String,
    #[prost(int64, tag = "8")]
    pub latency_ms: i64,
}

/// Decodes a notification request. Protobuf is the contract; a payload that starts
/// with `{` is accepted as JSON so a request can be hand-crafted for testing.
pub fn parse_requested(payload: &[u8]) -> Result<crate::NotificationRequested, String> {
    if payload.iter().find(|b| !b.is_ascii_whitespace()) == Some(&b'{') {
        return serde_json::from_slice(payload).map_err(|e| e.to_string());
    }

    let m = RequestedMsg::decode(payload).map_err(|e| e.to_string())?;
    let meta = m.meta.unwrap_or_default();
    if m.alert_id.is_empty() || meta.idempotency_key.is_empty() {
        return Err("missing alert_id or meta.idempotency_key".into());
    }
    Ok(crate::NotificationRequested {
        alert_id: m.alert_id,
        idempotency_key: meta.idempotency_key,
        channel_kind: m.channel_kind,
        recipient: m.recipient,
        subject: m.subject,
        body: m.body,
        attempt: m.attempt,
        tenant_id: meta.tenant_id,
        channel_id: m.channel_id,
    })
}

/// Encodes the final outcome of one notification for `notifications.delivered`.
pub fn encode_delivered(n: &crate::NotificationRequested, o: &crate::Outcome) -> Vec<u8> {
    let now_ms = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0);

    DeliveredMsg {
        meta: Some(EventMeta {
            event_id: uuid::Uuid::new_v4().to_string(),
            tenant_id: n.tenant_id.clone(),
            source: "notifier".into(),
            occurred_at: now_ms,
            traceparent: String::new(),
            idempotency_key: format!("delivered:{}", n.idempotency_key),
        }),
        alert_id: n.alert_id.clone(),
        channel_id: n.channel_id.clone(),
        success: o.success,
        attempt: o.attempt,
        status_code: o.status_code.unwrap_or(0),
        error: o.error.clone().unwrap_or_default(),
        latency_ms: o.latency_ms,
    }
    .encode_to_vec()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample() -> RequestedMsg {
        RequestedMsg {
            meta: Some(EventMeta {
                tenant_id: "t1".into(),
                idempotency_key: "k1".into(),
                ..Default::default()
            }),
            alert_id: "a1".into(),
            channel_id: "c1".into(),
            channel_kind: "email".into(),
            recipient: "ops@example.test".into(),
            subject: "subject".into(),
            body: "body".into(),
            attempt: 1,
        }
    }

    #[test]
    fn protobuf_request_round_trips() {
        let parsed = parse_requested(&sample().encode_to_vec()).unwrap();
        assert_eq!(parsed.alert_id, "a1");
        assert_eq!(parsed.idempotency_key, "k1");
        assert_eq!(parsed.tenant_id, "t1");
        assert_eq!(parsed.channel_id, "c1");
        assert_eq!(parsed.channel_kind, "email");
        assert_eq!(parsed.attempt, 1);
    }

    #[test]
    fn json_request_is_still_accepted() {
        let json = br#"{"alertId":"a2","idempotencyKey":"k2","channelKind":"webhook","recipient":"http://x","subject":"s","body":"{}","attempt":1}"#;
        let parsed = parse_requested(json).unwrap();
        assert_eq!(parsed.alert_id, "a2");
        assert_eq!(parsed.channel_id, "");
    }

    #[test]
    fn request_without_an_idempotency_key_is_rejected() {
        let mut m = sample();
        m.meta = None;
        assert!(parse_requested(&m.encode_to_vec()).is_err());
    }

    #[test]
    fn garbage_is_rejected() {
        assert!(parse_requested(&[0xff, 0xff, 0xff]).is_err());
    }

    #[test]
    fn delivered_event_carries_the_outcome() {
        let n = parse_requested(&sample().encode_to_vec()).unwrap();
        let o = crate::Outcome {
            success: false,
            attempt: 5,
            status_code: Some(503),
            error: Some("non-2xx status: 503".into()),
            latency_ms: 42,
        };
        let back = DeliveredMsg::decode(encode_delivered(&n, &o).as_slice()).unwrap();
        assert!(!back.success);
        assert_eq!(back.attempt, 5);
        assert_eq!(back.status_code, 503);
        assert_eq!(back.alert_id, "a1");
        assert_eq!(back.meta.unwrap().idempotency_key, "delivered:k1");
    }
}
