# notifier

Consumes `trackline.notifications.requested`, delivers via email or webhook
(SMS is Phase 2), and retries with jittered exponential backoff before giving up
to a dead-letter queue. See `apps/_contracts/kafka-topics.md` and
`apps/_contracts/auth.md` §7.

## The three lessons this service exists to teach

1. **Idempotency.** `idempotency_key` is checked before any delivery is
   attempted. Without it, a redelivered Kafka message (which *will* happen —
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

| Variable | Default | Meaning |
|---|---|---|
| `KAFKA_BROKERS` | `kafka:9092` | Broker list |
| `SMTP_HOST` | `mailpit` | Never a real provider outside prod |
| `SMTP_PORT` | `1025` | Mailpit's default SMTP port |
| `SMTP_FROM` | `alerts@trackline.local` | From address |
| `WEBHOOK_TIMEOUT_SECS` | `5` | Per `limits.md` §2 tier webhook timeouts (Pro default used as the fallback) |

## Building it

```bash
cd apps/notifier
cargo build
cargo test
```

**Not yet run.** No Rust toolchain confirmed, no Kafka broker, no Mailpit
container yet. `rdkafka` needs `librdkafka` (via the `cmake-build` feature) —
expect the first build to be slow while it compiles that dependency from
source; this is normal, not a failure.

## What's deliberately not implemented yet

- SMS channel — needs a real provider sandbox (Phase 2)
- Protobuf decoding of Kafka payloads — Phase 0 decodes JSON for simplicity;
  switching to `prost`-generated types once `buf generate` output exists is a
  contained follow-up
- Durable, shared idempotency store (Redis) — Phase 2, same reasoning as
  `ingest`'s batch dedup
- Per-tenant in-flight delivery caps (`limits.md` §7) — Phase 6, once load
  tests exist to validate against
