# Trackline — The Application

> Multi-tenant fleet telemetry SaaS. Customers register vehicle fleets; devices stream
> GPS + sensor telemetry; the platform ingests, aggregates, predicts ETAs, detects
> anomalies, alerts customers, bills them monthly, and renders a live map.

The application exists to create **operational pressure**. Every service below was chosen
because it forces a specific class of infrastructure problem.

---

## 1. Why this app and not a CRUD app

A CRUD app produces no interesting failures, so every chaos experiment and SLO becomes
theatre. Trackline is simultaneously:

- **Synchronous** (REST + gRPC APIs with latency SLOs)
- **Asynchronous** (queues, retries, DLQs, idempotency)
- **Streaming** (stateful windowed aggregation, consumer lag, exactly-once)
- **Batch** (nightly billing — correctness matters, "money bugs")
- **Real-time** (WebSocket live map — freshness SLO)
- **ML** (heavy compute, separate node pools, model artifacts)
- **Multi-tenant** (isolation, quotas, noisy neighbours, per-tenant cost)
- **Stateful** (Postgres, Kafka, Redis, time-series, object storage)

It generates real failure modes without being told to: backpressure collapse, hot
partitions, cache stampede, thundering herd, connection-pool exhaustion, poison messages,
slow queries under load.

---

## 2. Services

### Phase 0 core (built first)

| Service | Lang | Responsibility | Infrastructure pressure it creates |
|---|---|---|---|
| `edge-gateway` | Go | Public API surface, authN/Z, rate limiting, tenant routing | Ingress / Gateway API, TLS + cert rotation, WAF, mTLS, rate-limit state in Redis |
| `ingest` | Go | High-rate telemetry sink (gRPC + MQTT) → Kafka | HPA/KEDA on lag, backpressure, load shedding, partition strategy |
| `fleet-api` | TypeScript (NestJS) | Tenants, fleets, vehicles, drivers, geofences (CRUD) | Postgres ops, schema migrations, pool exhaustion, N+1, slow-query tuning |
| `notifier` | Rust | Webhooks / email / SMS on alert events | Queue semantics, retries with jitter, idempotency keys, DLQ, 3rd-party outage handling |
| `web` | Next.js / React | Dashboard + live map + tenant admin | CDN, static hosting, WebSockets, RUM, cache invalidation |
| `device-sim` | Go | Synthetic fleet: 10k → 1M devices, scriptable behaviours | Your load generator, chaos driver, and cost driver |

### Phase 2 additions

| Service | Lang | Responsibility | Infrastructure pressure |
|---|---|---|---|
| `stream-proc` | Java (Flink) | Windowed aggregation, geofence entry/exit, speeding + idle detection | Stateful streaming, checkpointing, savepoints, rebalance storms, exactly-once |
| `eta-ml` | Python (FastAPI) | ETA prediction + anomaly scoring; batch retraining | GPU/high-CPU node pools, taints/tolerations, model registry, artifact storage |
| `billing-batch` | Java (Spring Batch) | Nightly usage rollup → invoices | CronJob failure semantics, correctness, idempotent reruns, financial audit trail |

---

## 3. Data layer

| Store | Role | Phase | Ops skills it forces |
|---|---|---|---|
| **PostgreSQL** | System of record (tenants, fleets, vehicles, invoices) | 0 | Migrations, pooling (PgBouncer), replication, PITR, zero-downtime schema change |
| **Redis** | Hot cache, rate limiting, session, WebSocket fan-out | 0 | Eviction policy, cache stampede, persistence tradeoffs, cluster mode |
| **Kafka** | Event backbone (telemetry, alerts, billing events) | 0 | Partitions, consumer groups, lag, retention, rebalancing, tuning |
| **Object storage** (S3 / R2) | Cold telemetry archive, ML artifacts, backups | 0 | Lifecycle policies, versioning, encryption, cross-region replication, egress cost |
| **TimescaleDB / Influx** | Time-series telemetry queries | 2 | Hypertables, compression, retention, downsampling |
| **ClickHouse / BigQuery** | Tenant analytics + cost attribution | 2 | Columnar modelling, query cost, data gravity across clouds |

---

## 4. Tenancy model

Three tenant tiers — this is what makes isolation, quotas and FinOps real:

| Tier | Isolation | Quota | Exists to teach |
|---|---|---|---|
| **Free** | Shared everything, shared schema | 50 devices, 1 Hz | Noisy neighbours, fair-share rate limiting, load shedding |
| **Pro** | Shared cluster, dedicated Postgres schema, dedicated Kafka partitions | 5k devices, 1 Hz | Per-tenant SLOs, quota enforcement, per-tenant dashboards |
| **Enterprise** | Dedicated namespace, dedicated DB, optional dedicated region | 500k devices, 10 Hz | Namespace isolation, network policy, per-tenant cost showback, data residency |

---

## 5. Candidate SLIs and SLOs

Draft targets — refined in Phase 3 with real data.

| # | SLI | Proposed SLO | Why it's interesting |
|---|---|---|---|
| 1 | `fleet-api` read availability | 99.9% over 30d | Classic availability SLO; drives error budget policy |
| 2 | `fleet-api` read latency | p99 < 300 ms | Exposes DB tuning, pooling, cache design |
| 3 | Telemetry ingest success rate | 99.95% | Forces load-shedding vs. dropping-data tradeoff |
| 4 | **Live position freshness** | p99 device→map < 5 s | End-to-end pipeline SLO spanning 4 services — the hard one |
| 5 | Alert delivery latency | p95 < 30 s from event | Depends on a 3rd party; teaches dependency SLOs |
| 6 | Billing correctness | 100% of invoices reconcile | A "correctness SLO" — no error budget allowed |
| 7 | Deployment success rate | > 95% without rollback | A *delivery* SLI, not a runtime one |

---

## 6. Deliberate weaknesses (built in on purpose)

The app ships with realistic flaws so you have something real to find and fix. They are
**not** disclosed up front — you find them via observability, load testing, and chaos.
Categories seeded:

- A query that degrades non-linearly past a data volume threshold
- An unbounded in-memory buffer that fails only under sustained backpressure
- A cache with no jitter on TTL (stampede at scale)
- A retry path with no idempotency guard (duplicate side effects)
- A dependency with no timeout (cascading hang)
- A metric with unbounded cardinality (will melt Prometheus)

Finding each of these is a logged exercise in `ops/postmortems/`.

---

## 7. Contracts and interfaces

| Interface | Format | Where |
|---|---|---|
| Public REST API | OpenAPI 3.1 | `apps/edge-gateway/api/openapi.yaml` |
| Internal service-to-service | gRPC + protobuf | `apps/_proto/` |
| Telemetry ingest | protobuf over gRPC; MQTT for constrained devices | `apps/_proto/telemetry.proto` |
| Events on Kafka | CloudEvents + Avro, Schema Registry enforced | `apps/_proto/events/` |
| Live map push | WebSocket, JSON frames | `apps/web/` |

Contract tests (Pact) between `web` ↔ `edge-gateway` ↔ `fleet-api` are a Phase 2 CI gate.

---

## 8. Local development target

`make up` must give a full working Trackline on the Mac via k3d + Tilt, with:

- All Phase 0 services hot-reloading
- Postgres + Redis + Kafka (single-broker, KRaft mode) running
- `device-sim` producing a 500-device fleet
- Prometheus + Grafana + OTel Collector + Loki + Tempo (slim profile)
- Reachable on a real domain over real TLS via Cloudflare Tunnel

Memory budget on a 16 GB arm64 Mac is the binding constraint — profiles (`slim`,
`full`) exist so the o11y stack can be toggled off.
