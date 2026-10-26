# stream-proc

The Phase 2 stateful stream processor (Java, Apache Flink 1.20). It reads
`trackline.telemetry.raw` and produces the same events, with the same keys,
headers and idempotency keys, as the Phase 0 `detector` role in `apps/pipeline`.
**Run one or the other, not both** — remove `detector` from the pipeline's `ROLES`
when this job is running.

| Output topic       | Message                                                                | When                                                                                                       |
| ------------------ | ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `events.geofence`  | `GeofenceEvent`                                                        | enter, exit, and dwell (once, after the geofence's dwell threshold)                                        |
| `events.violation` | `SpeedingEvent` / `IdleEvent`, told apart by the `x-event-type` header | over the limit for 10 s, repeating every 10 s; ignition on and under 2 km/h for 60 s, repeating every 60 s |
| `events.vehicle`   | `VehicleOfflineEvent`                                                  | silent for 60 s, repeating every 60 s                                                                      |
| `usage.recorded`   | `UsageRecorded`                                                        | one per tenant per 30 s event-time window: points and distinct devices                                     |

The thresholds are deliberately lower than any tenant's rule: `alert-engine` applies
rule parameters (tolerance, minimum duration, cooldown); this job only emits the
base events.

## Why Flink, and what it is practising

- **Keyed state.** One `VehicleState` per vehicle (current geofences, speeding and
  idle episodes, last-seen) in Flink keyed state, checkpointed. A restart resumes
  mid-geofence instead of forgetting where every vehicle was.
- **Timers.** Offline detection is a processing-time timer per vehicle; a vehicle
  that stops sending produces no elements, so nothing else would notice.
- **Event-time windows.** Usage is windowed on _ingest_ time with 10 s of allowed
  out-of-orderness, so a device with a wrong clock cannot push usage into another day.
- **Replay safety.** Event idempotency keys are deterministic
  (`geofence:{vehicle}:{geofence}:{transition}:{ms}`, `usage:{tenant}:{windowStart}`),
  so reprocessing after a restart or a savepoint restore produces keys the consumers
  already know.
- **Delivery.** At-least-once by default. `EXACTLY_ONCE=true` switches the Kafka sink
  to transactions (the checkpoint interval must be shorter than the broker's
  transaction timeout).

## Configuration

| Variable                 | Default            | Meaning                                                                                               |
| ------------------------ | ------------------ | ----------------------------------------------------------------------------------------------------- |
| `KAFKA_BROKERS`          | `kafka:9092`       | Broker list                                                                                           |
| `KAFKA_GROUP_ID`         | `stream-processor` | Offsets are committed under this group on checkpoint                                                  |
| `DATABASE_URL`           | — (required)       | `postgres://user:pass@host:port/db`; the device and geofence registry is read from here and refreshed |
| `STARTING_OFFSETS`       | `latest`           | `latest`, `earliest`, or `committed` on first start                                                   |
| `CHECKPOINT_INTERVAL_MS` | `30000`            | Checkpoint interval                                                                                   |
| `CHECKPOINT_DIR`         | —                  | Where checkpoints go (`file://…`, `s3://…`); unset keeps them in memory                               |
| `PARALLELISM`            | `1`                | Job parallelism                                                                                       |
| `USAGE_WINDOW_SECONDS`   | `30`               | Usage window length                                                                                   |
| `REGISTRY_TTL_SECONDS`   | `30`               | Registry snapshot lifetime                                                                            |
| `EXACTLY_ONCE`           | `false`            | Transactional Kafka sink                                                                              |

If Postgres is unreachable the job keeps serving its last registry snapshot and
retries; it does not stop.

## Build and test

The protobuf sources are read from `../_proto`, so build from a tree that has
`apps/` in it (Docker context `apps/`).

```bash
cd apps/stream-proc
mvn package                 # compile, run the tests, build target/stream-proc-0.1.0.jar
mvn package -Pcluster       # same, with Flink itself `provided`, for submitting to a cluster
java -jar target/stream-proc-0.1.0.jar     # standalone: runs in an embedded Flink mini cluster
```

`mvn` needs Java 17. No Java toolchain is assumed on the workstation; the build was
verified in `maven:3.9-eclipse-temurin-17` and run on `eclipse-temurin:17-jre`.

The fat jar's manifest carries the `Add-Opens` entries Flink's serialisation needs on
Java 17, so `java -jar` works without extra flags. A cluster supplies its own.

## Verified

The detector state machine has 13 unit tests (baseline vs transition, dwell, speeding
and idle episodes, redelivered and out-of-order points, clock skew, offline repeat,
encoding contract, deterministic usage keys). End to end against a throwaway Kafka and
Postgres: with the pipeline's `detector` role off, the job's speeding events became
alerts through `alert-engine`, and its usage windows arrived on `usage.recorded`.

## What's deliberately not implemented

- Savepoint/upgrade procedure, state migration, and a Flink Kubernetes Operator
  manifest: that is the platform half of this service.
- Metrics (Flink's own reporters are the Phase 3 hook).
