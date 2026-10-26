# ingest

The telemetry sink. Devices stream GPS points here over gRPC; every valid point
is published to Kafka's `trackline.telemetry.raw` topic and the service holds no
state of its own. See `apps/_contracts/kafka-topics.md`.

## Why it holds no state

Statelessness is what lets you run ten copies behind a load balancer with no
coordination. All the interesting state — last known position, history,
aggregation — lives in consumers downstream (`position-updater`,
`telemetry-archiver`, `stream-proc`), not here. If `ingest` needed shared state,
scaling it would require a distributed lock, and that's a much harder problem than
the one it's solving.

The optional capabilities below keep that property: the only shared state is
**read** from Redis (the device registry, which `apps/pipeline`'s `registry-sync`
role maintains) or **written** to it as a cache (batch ids). A replica can be killed
or added at any time.

## Delivery guarantee

`kafka.RequireAll` + a single in-flight write per call means a lost telemetry
point becomes a returned error, not a silent gap. `PublishBatch`'s `batch_id` is
de-duplicated so a retried batch is not ingested twice: in memory by default, and
in Redis (shared across replicas and surviving restarts, falling back to memory if
Redis is unreachable) when `REDIS_ADDR` is set.

## Optional capabilities

Everything here is off unless its environment variable turns it on, so the plain
Compose setup keeps working unchanged.

| Capability                | Turned on by                                | What it does                                                                                                                                                                                                                                                                                             |
| ------------------------- | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Device registry**       | `REDIS_ADDR`                                | Resolves each device to its tenant, vehicle, tier and rate limit from the registry the pipeline publishes. Events are then keyed by `vehicle_id` (`kafka-topics.md` §2) and carry the tenant. Lookups are cached 30 s (5 s for unknown devices) and fall back to the last known answer if Redis is down. |
| **Device authentication** | `DEVICE_AUTH=required` (needs `REDIS_ADDR`) | Points from unknown devices, or whose `authorization: Bearer <token>` does not match the device's stored SHA-256 (`auth.md` §5), are rejected. Revocation is eventually consistent, bounded by the cache TTL.                                                                                            |
| **Per-device rate limit** | `REDIS_ADDR`                                | In-process token bucket at the tenant's `max_hz` (burst 5); over-rate points are rejected with a `retry_after_ms`, never an RPC error (`limits.md` §5).                                                                                                                                                  |
| **Load shedding**         | `LOAD_SHEDDING=on`                          | Watches publish backlog, latency and error rate and sheds in the documented order — sample Free, reject Free, sample Pro, reject Pro — with Enterprise never shed (`limits.md` §6). `SHED_CAPACITY` sets what "backlog at 80%" means.                                                                    |
| **MQTT transport**        | `MQTT_BROKER_URL`                           | Subscribes to `trackline/devices/<device id>/telemetry` (payload: a protobuf `PublishBatchRequest`) and feeds the same pipeline as gRPC.                                                                                                                                                                 |

On MQTT: **authentication is the broker's job.** Its ACL must let a device publish
only to its own topic; `ingest` trusts the device id in the _topic_, never the one in
the payload. The broker itself is not part of this repository. `MQTT_SHARED_GROUP`
makes replicas share the subscription (`$share/<group>/…`) on brokers that support it.

## Environment variables

| Variable                          | Default            | Meaning                                                        |
| --------------------------------- | ------------------ | -------------------------------------------------------------- |
| `LISTEN_ADDR`                     | `:9090`            | gRPC listen address                                            |
| `KAFKA_BROKERS`                   | `kafka:9092`       | Comma-separated broker list                                    |
| `REDIS_ADDR`                      | —                  | Enables the registry, rate limits and Redis-backed batch dedup |
| `DEVICE_AUTH`                     | `off`              | `required` rejects unknown devices and bad tokens              |
| `LOAD_SHEDDING`                   | `off`              | `on` enables tier-aware shedding                               |
| `SHED_CAPACITY`                   | `2000`             | In-flight publishes that count as a full backlog               |
| `MQTT_BROKER_URL`                 | —                  | e.g. `tcp://mosquitto:1883`; enables the MQTT transport        |
| `MQTT_CLIENT_ID`                  | `trackline-ingest` |                                                                |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | —                  | Broker credentials, from the secret store                      |
| `MQTT_SHARED_GROUP`               | —                  | Shared-subscription group                                      |

## Building it

Requires generated code from the proto contracts:

```bash
cd apps/_proto && buf generate      # produces apps/gen/go/trackline/v1/*
cd ../ingest && go test ./... && go run ./cmd/ingest
```

The module is pinned to `go 1.23` with dependency versions that build on the Go
1.25 image; keep it that way when updating. The unit tests cover the shedding policy
per tier (including "Enterprise is never shed"), the token bucket, token hashing,
de-duplication, MQTT topic parsing and the throttle path. End to end, 500 devices
with `DEVICE_AUTH=required` were accepted through the pipeline.

## What's deliberately not implemented yet

- Verifying the MQTT path against a real broker — the transport is written and its
  parsing tested, but no broker was available to run it against
- Weighted fair queueing across tenants (`limits.md` §7) — Phase 6, needs load tests
  to tune against
