# pipeline

The Kafka consumers that turn telemetry into the rest of the system: last-known
positions, the telemetry archive, geofence/violation/offline events, alerts,
notification bookkeeping and usage. It is **one Go binary that hosts several
independent consumer groups** ("roles"), chosen with `ROLES`, so each can be run
and scaled on its own lag (`apps/_contracts/kafka-topics.md` §4).

It is not in the original six-service layout: `kafka-topics.md` names these
consumers but nothing delivered them, and without them telemetry stops at Kafka.

## Roles

| Role (`ROLES`)       | Consumer group       | Reads                                                   | Does                                                                                                                                                                                                                                   |
| -------------------- | -------------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `registry-sync`      | — (timer)            | Postgres                                                | Publishes the device registry to Redis `registry:devices` for `ingest` (auth, per-tier rate, shedding). **Run exactly one replica.**                                                                                                   |
| `position-updater`   | `position-updater`   | `telemetry.raw`                                         | Upserts `vehicle_position_latest` (last write wins by `recorded_at`), touches `device.last_seen_at`, publishes each position to Redis `positions:{tenantId}` for the WebSocket map. Unknown devices go to `telemetry.dlq` immediately. |
| `telemetry-archiver` | `telemetry-archiver` | `telemetry.raw`                                         | Batch-appends to the `telemetry` table.                                                                                                                                                                                                |
| `detector`           | `detector`           | `telemetry.raw`                                         | **Phase 0 stand-in for `apps/stream-proc`.** Geofence enter/exit/dwell, speeding, idle, offline, and per-tenant usage. Same output as the Flink job; run one or the other, not both.                                                   |
| `alert-engine`       | `alert-engine`       | `events.geofence`, `events.violation`, `events.vehicle` | Applies alert rules (tolerance, minimum duration, cooldown), writes `alert` and `notification_delivery` rows, publishes `alerts.raised` and `notifications.requested`.                                                                 |
| `delivery-recorder`  | `delivery-recorder`  | `notifications.delivered`                               | Moves `notification_delivery` to `sent` / `failed` / `dead`.                                                                                                                                                                           |
| `usage-collector`    | `usage-collector`    | `usage.recorded`                                        | Adds usage deltas into `usage_daily`; a repeated message is dropped by its idempotency key.                                                                                                                                            |

With `ROLES` unset every role runs in the one process, which is what the Phase 0
Compose stack wants. Splitting them is a deployment decision: same image, different
`ROLES`.

## Delivery semantics

At-least-once, manual commit after the batch is processed. Every write is
idempotent (upserts, unique idempotency keys, deterministic alert ids), so a
replay changes nothing.

Permanent failures (schema mismatch, unknown device) are dead-lettered at once.
Transient failures are retried with exponential backoff and jitter; for roles with
a DLQ the message is then dead-lettered, and for roles **without** one
(`alert-engine`, `delivery-recorder`, `usage-collector`) the batch is retried
without committing rather than dropped — losing an alert or a usage record is worse
than a stalled partition, and the stall is visible as consumer lag.

## Environment variables

| Variable               | Default      | Meaning                                                      |
| ---------------------- | ------------ | ------------------------------------------------------------ |
| `KAFKA_BROKERS`        | `kafka:9092` | Comma-separated broker list                                  |
| `DATABASE_URL`         | — (required) | Postgres connection string                                   |
| `REDIS_ADDR`           | `redis:6379` | Registry hash and position pub/sub                           |
| `ROLES`                | all          | Comma-separated subset of the roles above                    |
| `GROUP_SUFFIX`         | —            | Appended to every consumer group id (side-by-side test runs) |
| `REGISTRY_TTL_SECONDS` | `60`         | How long the in-process registry snapshot lives              |
| `HEALTH_ADDR`          | `:8082`      | `/healthz` (liveness) and `/readyz` (Postgres reachable)     |

## Building and running

Like `ingest` and `device-sim`, it builds with `apps/` as the context because it
imports the generated protobuf module through `replace ... => ../gen/go`.

```bash
cd apps/_proto && buf generate
cd ../pipeline && go test ./... && go run ./cmd/pipeline
```

The module is pinned to `go 1.23` with dependency versions that build on the Go
1.25 image the other services use; keep it that way when updating.

## Verified

Unit tests cover the detector state machine, alert rules and cooldown, geometry
and the consumer retry loop. End to end against a throwaway Kafka/Postgres/Redis:
500 simulated devices produced roughly 22,000 telemetry rows in 45 s, every
vehicle's position was updated, usage rows were written, and the scripted
scenarios raised their alerts.

## What's deliberately not implemented

- Metrics, traces and the OTel SDK — Phase 3 instrumentation is yours.
- Object-storage archive for telemetry; `telemetry-archiver` writes Postgres only.
