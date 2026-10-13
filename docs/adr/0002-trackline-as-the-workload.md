# ADR-0002: Trackline as the workload for HELIOS

**Status:** Accepted
**Date:** 2026-09-16

## Context

HELIOS exists to build competence across DevOps, SRE, Platform Engineering, and SecOps.
Those disciplines are only exercisable against a workload that produces real operational
pressure. The workload choice therefore determines the ceiling of what can be learned.

A workload that is purely CRUD produces no interesting failure modes. Chaos experiments
become theatre, SLOs become arbitrary, capacity planning becomes trivial, and autoscaling
has nothing meaningful to react to.

## Decision

Build **Trackline** — a multi-tenant fleet telemetry SaaS — as the sole workload.

It is deliberately constructed to be simultaneously synchronous, asynchronous, streaming,
batch, real-time, ML-serving, multi-tenant, and stateful. It ships with a built-in load
generator (`device-sim`) capable of scaling from hundreds to a million simulated devices.

The application also ships with **seeded weaknesses** — a non-linearly degrading query, an
unbounded buffer, a cache without TTL jitter, a non-idempotent retry path, a missing
timeout, and an unbounded-cardinality metric. These are not disclosed in advance and must
be discovered through observability, load testing, and chaos.

## Alternatives considered

| Alternative | Why it lost |
|---|---|
| A standard e-commerce / microservices demo app | Well-trodden, no streaming or batch tier, failure modes are synthetic |
| Several small unrelated apps | No end-to-end SLO is possible; no cross-service tracing story; no shared data gravity |
| An existing open-source app (e.g. a public reference architecture) | Cannot seed deliberate weaknesses; no control over the failure surface; less understanding of internals |
| A data/ML-only pipeline | No latency SLO, no real-time tier, weak ingress/edge/security story |

## Consequences

**Easier**
- Every technology in the matrix has a concrete reason to exist.
- SLOs are derived from user-visible behaviour rather than invented.
- Chaos and load testing produce genuine, reproducible failures.
- Per-tenant cost attribution, quota enforcement, and isolation become real problems.

**Harder**
- The application is large. Phase 0 is trimmed to five core services to prevent the
  workload itself from becoming the bottleneck.
- Polyglot services (Go, TypeScript, Rust, Python, Java) increase build and supply-chain
  complexity — which is intentional, but front-loads CI effort.
- Local development on a 16 GB arm64 machine is memory-constrained; profiles are required.

**Accepted risk**
- Time may be spent debugging application behaviour rather than infrastructure. This is
  mitigated by the division of labour: application source is provided, not authored.
