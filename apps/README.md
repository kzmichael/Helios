# Trackline — Application Source

Delivered source for the HELIOS workload. **This directory is application development,
not platform work** — read it, build it, break it, but the infrastructure around it is
yours to design.

See `docs/01_APPLICATION.md` for what each service exists to stress.

---

## Layout

```
apps/
├── _contracts/          OpenAPI 3.1 spec, auth/limits/Kafka contracts
├── _proto/              Protobuf: internal gRPC + Kafka event schemas
├── _db/
│   ├── migrations/      Schema, forward-only, numbered
│   ├── seed/            Deterministic seed data
│   └── load/            Opt-in extra fleet (350 vehicles) for 500-device runs
├── edge-gateway/        Go    — public API, authN/Z, rate limiting
├── ingest/              Go    — telemetry sink (gRPC + MQTT)
├── pipeline/            Go    — Kafka consumers: positions, archive, detection, alerts, usage
├── fleet-api/           TS    — tenants, fleets, vehicles, drivers, geofences, alerts, billing reads
├── notifier/            Rust  — webhooks, email, SMS
├── web/                 Next  — dashboard, live map, vehicles, history
├── device-sim/          Go    — synthetic fleet load generator
├── stream-proc/         Java  — Phase 2: Flink job (the stateful alternative to pipeline's detector)
├── billing-batch/       Java  — Phase 2: Spring Batch usage -> invoices
└── eta-ml/              Python — Phase 2: ETA prediction and anomaly scoring
```

**[ANNOTATIONS.md](ANNOTATIONS.md)** collects every non-obvious "why" comment from
the original six services in one place — read it when something in the code looks
arbitrary and you don't want to hunt through several codebases to find the reasoning.

---

## Delivery status

Every component below is implemented. "Verified" says how far each was actually run,
which is not the same as the Compose stack working — that wiring is yours.

| Component                        | State                                                                     | Verified                                                                          |
| -------------------------------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `_proto/`, `_contracts/`, `_db/` | Delivered (V3 migration and the opt-in load fleet added)                  | Applied to a throwaway Postgres                                                   |
| `device-sim/`                    | Complete: seeded fleet identities, scenarios, chaos                       | Unit tests; 500 devices through `ingest`                                          |
| `ingest/`                        | Complete: registry, device auth, rate limits, shedding, Redis dedup, MQTT | Unit tests; end to end with auth required. **MQTT not run against a broker**      |
| `pipeline/`                      | New. All seven consumer roles                                             | Unit tests; end to end                                                            |
| `edge-gateway/`                  | Complete: JWT, API keys, WebSocket token, body cap, `429` shape           | Unit tests; end to end through to `fleet-api`                                     |
| `fleet-api/`                     | Complete for the OpenAPI contract plus the live WebSocket                 | Jest specs; every endpoint exercised through the gateway                          |
| `notifier/`                      | Complete: protobuf and JSON, email, webhook, SMS, delivered events        | Unit tests; emails delivered to Mailpit (built in a container)                    |
| `web/`                           | Complete: live Leaflet map, vehicles, history, generated API types        | Production build; checked in a browser                                            |
| `stream-proc/`                   | Complete                                                                  | Unit tests; end to end in a container (alerts raised from its events)             |
| `billing-batch/`                 | Complete                                                                  | Unit tests; end to end incl. idempotent rerun and a forced reconciliation failure |
| `eta-ml/`                        | Complete                                                                  | 16 tests; trained and served from simulated telemetry                             |

Nothing here has been built as a container image or run under Compose: Dockerfiles for
`pipeline`, `stream-proc`, `billing-batch` and `eta-ml` do not exist, and the existing
ones need the new environment variables listed in each service's README. Dependencies
still resolve from `go.mod` / `pnpm-lock.yaml` / `Cargo.lock` / `pom.xml` /
`requirements.txt`; generated protobuf code from `buf generate` is still not committed.

Each service's README ends with what it deliberately leaves out.

---

## Contracts are the source of truth

Six contracts define every boundary in the system. Generated code is **not committed** —
generate it, don't edit it.

| Boundary           | Contract                              | Consumers                                         |
| ------------------ | ------------------------------------- | ------------------------------------------------- |
| Public REST        | `_contracts/openapi.yaml`             | `web`, customer integrations                      |
| Identity & tenancy | `_contracts/auth.md`                  | every service, and the platform                   |
| Quotas & shedding  | `_contracts/limits.md`                | `edge-gateway`, `ingest`, `fleet-api`             |
| Event backbone     | `_contracts/kafka-topics.md`          | every producer and consumer                       |
| Internal gRPC      | `_proto/trackline/v1/telemetry.proto` | `device-sim` → `ingest`                           |
| Kafka payloads     | `_proto/trackline/v1/events.proto`    | `ingest` → `notifier`, billing, stream processing |

```bash
cd apps/_proto && buf lint && buf generate
```

If you change a contract, both sides must be regenerated. Phase 2 adds contract tests to
catch the case where they aren't.

The three Markdown contracts are **not documentation** — they are requirements the
platform must satisfy. `auth.md` §3 in particular describes a header-stripping rule whose
absence is a data breach.

---

## Data model in one paragraph

A **tenant** is a customer on one of three tiers. A tenant owns **fleets**, each holding
**vehicles**. A vehicle is paired with a **device** that emits telemetry. **Geofences**
and **alert rules** are evaluated against incoming telemetry; matches raise **alerts**,
which fan out to **notification channels** as **deliveries**. Telemetry is aggregated
daily into **usage** records, which become **invoices**. Every mutation writes an
**audit log** entry.

Multi-tenancy is by `tenant_id` column on every table. Enforcing that isolation is a
platform problem, not an application one — the application trusts the column.

---

## Prerequisites

| Tool       | Version | Used by                                            |
| ---------- | ------- | -------------------------------------------------- |
| PostgreSQL | 16+     | schema, seed                                       |
| `buf`      | 1.4x    | protobuf lint + codegen                            |
| Go         | 1.23+   | `edge-gateway`, `ingest`, `pipeline`, `device-sim` |
| Node       | 22+     | `fleet-api`, `web`                                 |
| Rust       | 1.8x+   | `notifier`                                         |
| Java       | 17      | `stream-proc`, `billing-batch` (Maven 3.9)         |
| Python     | 3.12+   | `eta-ml`                                           |

Pin these in `.mise.toml` — that is story `HEL-0.1.2`. The Go modules are deliberately
pinned to `go 1.23` with dependency versions that build on the Go 1.25 base image;
a newer toolchain's `go get` will quietly raise that, so check `go.mod` after updating.

---

## Applying schema and seed

Migrations are plain SQL, forward-only, numbered. They are **not** wired to a migration
tool — choosing and wiring one is story `HEL-0.4.2`.

```bash
psql "$DATABASE_URL" -f _db/migrations/V1__init.sql
psql "$DATABASE_URL" -f _db/seed/V2__seed_tenants.sql
psql "$DATABASE_URL" -f _db/migrations/V3__notification_delivery_idempotency.sql
psql "$DATABASE_URL" -f _db/load/load-fleet.sql       # optional: the 500-device fleet
```

`V3` adds the unique index that lets the alert engine insert a notification delivery
twice without creating two rows; the pipeline depends on it.

Seed data is deterministic: fixed UUIDs, fixed counts, reproducible on every run against
a clean database. It creates three tenants with deliberately uneven sizes — 5, 25, and
120 vehicles — because equal tenants teach nothing about noisy neighbours.

---

## A note on quality

This source is written to be realistic, not exemplary. It contains the kinds of decisions
real services contain, including some that will not survive contact with load. Finding
those is the point of Phases 3 and 6 — they are not disclosed here.
