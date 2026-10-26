# notifier

Consumes `trackline.notifications.requested`, delivers via email, webhook or SMS,
and retries with jittered exponential backoff before giving up to a dead-letter
queue. Every outcome is published to `trackline.notifications.delivered`, which
the pipeline's `delivery-recorder` turns into `notification_delivery` status. See
`apps/_contracts/kafka-topics.md` and `apps/_contracts/auth.md` §7.

## Message formats

The contract payload is protobuf (`NotificationRequested` in `events.proto`).
`src/proto.rs` decodes it with hand-written `prost` structs, so the build needs no
`protoc`; a payload that is not valid protobuf but is valid JSON of the same shape is
accepted too, which keeps hand-published test messages working. Anything else is
malformed and goes straight to the DLQ.

## Channels

| Kind      | Delivery                                                                   | Notes                                                                                                      |
| --------- | -------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `email`   | SMTP to `SMTP_HOST`                                                        | Mailpit everywhere before production                                                                       |
| `webhook` | POST, body signed per `auth.md` §7 (`HMAC-SHA256` over `"{t}.{raw_body}"`) | Needs `WEBHOOK_SECRET`; without it webhook deliveries are dead-lettered, loudly, rather than sent unsigned |
| `sms`     | POST to `SMS_GATEWAY_URL` (JSON `{to, from, text}`, optional basic auth)   | Point it at a local sink; no real provider is wired                                                        |

## The three lessons this service exists to teach

1. **Idempotency.** `idempotency_key` is checked before any delivery is
   attempted. Without it, a redelivered Kafka message (which _will_ happen —
   the system is at-least-once end to end) becomes a duplicate email or a
   duplicate webhook.
2. **Jittered backoff.** `retry::backoff_for_attempt` adds up to 30% random
   jitter on top of exponential backoff. Remove the jitter and every failed
   delivery for the same outage retries in lockstep — a self-inflicted retry
   storm the instant the downstream recovers.
3. **Never block a partition on a poison message.** Malformed payloads go
   straight to the DLQ with zero retries. A bounded number of retries, then
   give up — never retry forever in place.

## Why email goes to Mailpit, never a real inbox

Every environment before real production points `SMTP_HOST` at a local Mailpit
container. This protects the lab domain's sending reputation — see
`docs/04_ENVIRONMENTS.md` §9 — before the domain even exists.

## Environment variables

| Variable                                    | Default                   | Meaning                                                                                                              |
| ------------------------------------------- | ------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `KAFKA_BROKERS`                             | `kafka:9092`              | Broker list                                                                                                          |
| `KAFKA_GROUP_ID`                            | `notification-dispatcher` | Consumer group id (override for side-by-side runs)                                                                   |
| `SMTP_HOST`                                 | `mailpit`                 | Never a real provider outside prod                                                                                   |
| `SMTP_PORT`                                 | `1025`                    | Mailpit's default SMTP port                                                                                          |
| `SMTP_FROM`                                 | `alerts@trackline.local`  | From address                                                                                                         |
| `WEBHOOK_TIMEOUT_SECS`                      | `5`                       | Per `limits.md` §2 tier webhook timeouts (Pro default used as the fallback); also the SMS request timeout            |
| `WEBHOOK_SECRET`                            | —                         | Signing key for webhook deliveries; without it webhooks are dead-lettered. Comes from the secret store, never a file |
| `SMS_GATEWAY_URL`                           | —                         | Enables the SMS channel                                                                                              |
| `SMS_GATEWAY_USER` / `SMS_GATEWAY_PASSWORD` | —                         | Optional basic auth for the gateway                                                                                  |
| `SMS_FROM`                                  | `TRACKLINE`               | Sender id                                                                                                            |

## Building it

```bash
cd apps/notifier
cargo build
cargo test
```

`rdkafka` needs `librdkafka` built through its `cmake-build` feature, so the first
build is slow and needs `cmake` and a C toolchain; this is normal, not a failure. If
the workstation has no `cmake`, build in a `rust:1.88-bookworm` container (that is how
it was verified here). The unit tests (8) cover protobuf and JSON decoding, SMS
payloads and retry behaviour; an end-to-end run delivered alert emails into Mailpit.

## What's deliberately not implemented yet

- A real SMS provider integration — the channel speaks a generic JSON/HTTP contract
  and is pointed at a sink
- Durable, shared idempotency store (Redis) — Phase 2, same reasoning as
  `ingest`'s batch dedup
- Per-tenant in-flight delivery caps (`limits.md` §7) — Phase 6, once load
  tests exist to validate against
