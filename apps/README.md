# Trackline — Application Source

Delivered source for the HELIOS workload. **This directory is application development,
not platform work** — read it, build it, break it, but the infrastructure around it is
yours to design.

See `docs/01_APPLICATION.md` for what each service exists to stress.

---

## Layout

```
apps/
├── _contracts/          OpenAPI 3.1 spec for the public REST API
├── _proto/              Protobuf: internal gRPC + Kafka event schemas
├── _db/
│   ├── migrations/      Schema, forward-only, numbered
│   └── seed/            Deterministic seed data
├── edge-gateway/        Go    — public API, authZ, rate limiting
├── ingest/              Go    — telemetry sink (gRPC + MQTT)
├── fleet-api/           TS    — tenants, fleets, vehicles, geofences
├── notifier/            Rust  — webhooks, email, SMS
├── web/                 Next  — dashboard + live map
└── device-sim/          Go    — synthetic fleet load generator
```

**[ANNOTATIONS.md](ANNOTATIONS.md)** collects every non-obvious "why" comment from
the source above in one place — read it when something in the code looks
arbitrary and you don't want to hunt through six codebases to find the reasoning.

---

## Delivery status

| Component | State |
|---|---|
| `_proto/` | **Delivered** |
| `_contracts/openapi.yaml` | **Delivered** |
| `_contracts/auth.md` | **Delivered** |
| `_contracts/limits.md` | **Delivered** |
| `_contracts/kafka-topics.md` | **Delivered** |
| `_db/migrations/` | **Delivered** |
| `_db/seed/` | **Delivered** |
| `fleet-api/` | **Delivered (core)** — drivers, `/positions` write path, billing pending |
| `edge-gateway/` | **Delivered (core)** — API-key auth, shedding pending |
| `ingest/` | **Delivered (core)** — MQTT transport pending |
| `notifier/` | **Delivered (core)** — SMS channel pending |
| `device-sim/` | **Delivered (core)** — chaos injection pending |
| `web/` | **Delivered (core)** — realtime channel and map widget pending |

Every "core" service still requires: `go mod tidy` / `pnpm install` / `cargo build`
to resolve dependencies, generated protobuf code from `buf generate`, and the
runtime/compose stack from Phase 0's remaining stories. None of this has been
executed yet — see each service's own README for its specific gaps.

---

## Contracts are the source of truth

Six contracts define every boundary in the system. Generated code is **not committed** —
generate it, don't edit it.

| Boundary | Contract | Consumers |
|---|---|---|
| Public REST | `_contracts/openapi.yaml` | `web`, customer integrations |
| Identity & tenancy | `_contracts/auth.md` | every service, and the platform |
| Quotas & shedding | `_contracts/limits.md` | `edge-gateway`, `ingest`, `fleet-api` |
| Event backbone | `_contracts/kafka-topics.md` | every producer and consumer |
| Internal gRPC | `_proto/trackline/v1/telemetry.proto` | `device-sim` → `ingest` |
| Kafka payloads | `_proto/trackline/v1/events.proto` | `ingest` → `notifier`, billing, stream processing |

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

| Tool | Version | Used by |
|---|---|---|
| PostgreSQL | 16+ | schema, seed |
| `buf` | 1.4x | protobuf lint + codegen |
| Go | 1.23+ | `edge-gateway`, `ingest`, `device-sim` |
| Node | 22+ | `fleet-api`, `web` |
| Rust | 1.8x+ | `notifier` |

Pin these in `.mise.toml` — that is story `HEL-0.1.2`.

---

## Applying schema and seed

Migrations are plain SQL, forward-only, numbered. They are **not** wired to a migration
tool — choosing and wiring one is story `HEL-0.4.2`.

```bash
psql "$DATABASE_URL" -f _db/migrations/0001_init.sql
psql "$DATABASE_URL" -f _db/seed/0001_tenants.sql
```

Seed data is deterministic: fixed UUIDs, fixed counts, reproducible on every run against
a clean database. It creates three tenants with deliberately uneven sizes — 5, 25, and
120 vehicles — because equal tenants teach nothing about noisy neighbours.

---

## A note on quality

This source is written to be realistic, not exemplary. It contains the kinds of decisions
real services contain, including some that will not survive contact with load. Finding
those is the point of Phases 3 and 6 — they are not disclosed here.
