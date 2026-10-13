# Kafka Topology

Contract between the application and the platform. The application declares the topics it
needs and the guarantees it depends on; provisioning them is platform work.

---

## 1. Topics

| Topic | Key | Partitions (P0 / prod) | Retention | Cleanup | Producer | Consumers |
|---|---|---|---|---|---|---|
| `trackline.telemetry.raw` | `vehicle_id` | 1 / 24 | 24 h | delete | `ingest` | `stream-proc`, `position-updater`, archiver |
| `trackline.events.geofence` | `vehicle_id` | 1 / 6 | 7 d | delete | `stream-proc` | `alert-engine` |
| `trackline.events.violation` | `vehicle_id` | 1 / 6 | 7 d | delete | `stream-proc` | `alert-engine` |
| `trackline.events.vehicle` | `vehicle_id` | 1 / 3 | 7 d | delete | `stream-proc` | `alert-engine` |
| `trackline.alerts.raised` | `tenant_id` | 1 / 6 | 30 d | delete | `alert-engine` | `notifier`, `fleet-api` |
| `trackline.notifications.requested` | `alert_id` | 1 / 6 | 7 d | delete | `alert-engine` | `notifier` |
| `trackline.notifications.delivered` | `alert_id` | 1 / 3 | 7 d | delete | `notifier` | `fleet-api` |
| `trackline.usage.recorded` | `tenant_id` | 1 / 3 | 90 d | delete | `stream-proc` | `billing-batch` |
| `trackline.telemetry.dlq` | `vehicle_id` | 1 / 3 | 30 d | delete | any | operator, manually |
| `trackline.notifications.dlq` | `alert_id` | 1 / 3 | 30 d | delete | `notifier` | operator, manually |

Phase 0 runs a single broker in KRaft mode, so `replication.factor=1` and one partition
per topic. Multi-partition and multi-broker arrive with the cloud clusters.

---

## 2. Why these keys

Partition key determines two things: **ordering** and **hot-spotting**. Both matter.

**`vehicle_id` for telemetry.** Kafka guarantees ordering only within a partition. A
vehicle's positions arriving out of order produce a track that jumps backwards, corrupts
geofence transition detection, and generates false alerts. Keying by vehicle puts all of
one vehicle's points on one partition, in order.

**Not `tenant_id` for telemetry.** Tenants are deliberately unequal — the enterprise tenant
has 24× the vehicles of the free tenant. Keying by tenant would put 80% of all traffic on
one partition, and no amount of consumer scaling would help. That is a hot partition, and
it is the classic Kafka failure.

**`tenant_id` for alerts and usage.** These are consumed and aggregated per tenant, and
volume is low enough that skew does not matter. Ordering per tenant is what the consumer
needs.

**`alert_id` for notifications.** All delivery attempts for one alert must be processed in
order so that retry counting and DLQ decisions are coherent.

---

## 3. Delivery guarantees

| Property | Setting | Why |
|---|---|---|
| Producer acks | `acks=all` | A lost telemetry point is a gap in a customer's track |
| Idempotent producer | `enable.idempotence=true` | Prevents duplicates from producer-side retries |
| Max in-flight | `5` | Safe with idempotence enabled; preserves ordering |
| Consumer offsets | Manual commit **after** processing | At-least-once. Committing before processing loses data on crash |
| Consumer isolation | `read_committed` | Once transactions are introduced in Phase 2 |

The system is **at-least-once end to end**. Consumers must be idempotent. Exactly-once
delivery does not exist; exactly-once *processing* is achieved by making effects
idempotent, which is why `EventMeta.idempotency_key` is on every event.

---

## 4. Consumer groups

| Group | Topic | Notes |
|---|---|---|
| `position-updater` | `telemetry.raw` | Writes `vehicle_position_latest` and Redis. Last-write-wins by `recorded_at`. |
| `telemetry-archiver` | `telemetry.raw` | Batch-appends to the `telemetry` table, later to object storage |
| `stream-processor` | `telemetry.raw` | Stateful windowing; the only stateful consumer |
| `alert-engine` | `events.*` | Evaluates rules, applies per-rule cooldown |
| `notification-dispatcher` | `notifications.requested` | The retry path |
| `billing-collector` | `usage.recorded` | Feeds `usage_daily` |

**Consumer lag is the scaling signal, not CPU.** A consumer can be completely idle on CPU
while falling hours behind because it is blocked on a downstream call. Scaling on CPU in
that situation does nothing. KEDA scales these groups on lag — that is Phase 6.

---

## 5. Dead letter queues

A message goes to the DLQ when it fails processing after the configured attempts, or when
it fails schema validation.

| Failure | Attempts | Then |
|---|---|---|
| Transient (timeout, 5xx, broker unavailable) | 5, exponential backoff with jitter | DLQ |
| Permanent (schema mismatch, unknown tenant, malformed) | 0 | DLQ immediately |

**Never block a partition retrying a poison message.** One unparseable message must not
stop every vehicle behind it in the same partition. Retry with a bounded budget, then move
on.

DLQ depth is an alerting signal, and draining a DLQ is a runbook, not a script.

---

## 6. Schema evolution

Schemas live in `_proto/trackline/v1/`. Registry arrives in Phase 2 with `BACKWARD`
compatibility enforced.

Until then, the rules are enforced by discipline and `buf breaking`:

- Never reuse or renumber a field tag
- Never change a field's type
- New fields must be optional with sensible zero values
- Removed fields must be `reserved`
- Producers deploy **after** consumers can read the new schema

---

## 7. Sizing

Back-of-envelope, to make the retention numbers defensible:

```
1 telemetry point ≈ 120 bytes on the wire (protobuf)

150 devices  @ 1 Hz  →      18 KB/s  →   1.5 GB/day
10k devices  @ 1 Hz  →     1.2 MB/s  →   103 GB/day
1M devices   @ 1 Hz  →     120 MB/s  →    10 TB/day
```

At 24 h retention and a million devices, the raw topic alone needs ~10 TB. This is why
mega-scale load tests stay local or inside the VPC — and why `telemetry.raw` retention is
24 h while alerts keep 30 days.

Capacity planning against these numbers is Phase 6.

---

## 8. What the platform must provide

| Requirement | Phase |
|---|---|
| Topics created declaratively, not by auto-create | 1 |
| `auto.create.topics.enable=false` | 1 |
| Per-topic retention and partition counts as code | 1 |
| Consumer lag exported to Prometheus | 3 |
| Lag-based autoscaling | 6 |
| Credentials rotated, not static | 4 |
| Kafka self-hosted via Strimzi — **never managed** | all (cost) |
