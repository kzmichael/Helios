# Quotas, Rate Limits & Load Shedding

Contract between the application and the platform. Defines what a tenant is entitled to,
what happens when they exceed it, and what happens when the system itself is the
constraint.

---

## 1. Quota vs rate limit vs shedding

Three different mechanisms, routinely confused:

| | Question it answers | Scope | Response |
|---|---|---|---|
| **Quota** | "Are you entitled to this at all?" | Per tenant, per billing period | `422` — a business decision |
| **Rate limit** | "Are you going too fast right now?" | Per tenant, per second | `429` — a fairness decision |
| **Load shedding** | "Can *we* cope right now?" | System-wide | `503` — a survival decision |

Quota is about the contract. Rate limiting protects other tenants from one tenant. Load
shedding protects the system from everyone, including itself.

---

## 2. Tier entitlements

| | Free | Pro | Enterprise |
|---|---|---|---|
| Max devices | 50 | 5,000 | 500,000 |
| Max emission rate | 1 Hz | 1 Hz | 10 Hz |
| API requests | 60/min | 600/min | 6,000/min |
| Burst allowance | 2× for 10 s | 2× for 10 s | 2× for 30 s |
| History retention | 7 days | 90 days | 730 days |
| Alert rules | 5 | 50 | unlimited |
| Notification channels | 1 | 10 | unlimited |
| Webhook timeout | 3 s | 5 s | 10 s |
| Shedding priority | **first** | second | **last** |

Stored on the `tenant` row (`max_devices`, `max_hz`, `retention_days`); the rest derive
from `tier`.

---

## 3. Enforcement points

| Limit | Enforced at | Mechanism |
|---|---|---|
| Device count | `fleet-api`, on vehicle/device create | Transactional count check |
| API request rate | `edge-gateway` | Token bucket in Redis |
| Telemetry rate | `ingest`, per device | Token bucket, in-process |
| Payload size | `edge-gateway` | 1 MB body cap |
| Batch size | `ingest` | 1,000 points per `PublishBatch` |
| History query span | `fleet-api` | Capped by tier retention |
| Page size | `fleet-api` | 200 max, regardless of tier |

**Enforce at the earliest point that has the information.** Rate limiting at the gateway
means a rejected request never touches the database. Quota checks must be in `fleet-api`
because only it can count transactionally.

---

## 4. Algorithm: token bucket

Chosen over fixed-window and sliding-window for one reason: it permits bursts without
permitting sustained overload. A fixed window allows 2× the limit across a window
boundary; a sliding log is accurate but expensive at this request rate.

```
capacity   = burst allowance
refill     = limit / 60 tokens per second
key        = ratelimit:{tenant_id}:{route_class}
```

Implemented as an atomic Lua script in Redis so check-and-decrement cannot race.

**Failure mode:** if Redis is unavailable, `edge-gateway` **fails open** and serves the
request. Rejecting all traffic because the rate limiter is down converts a degradation
into an outage. This is a deliberate availability-over-fairness trade, and it must be
alerted on — a silently-disabled rate limiter is how one tenant takes down the platform.

---

## 5. Responses

### Rate limited

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 12
X-RateLimit-Limit: 600
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1789000912
Content-Type: application/problem+json

{ "type": "https://trackline.local/errors/rate-limited",
  "title": "Rate limit exceeded", "status": 429,
  "detail": "Tenant limit of 600 requests/min exceeded.",
  "traceId": "4bf92f…" }
```

### Quota exceeded

```http
HTTP/1.1 422 Unprocessable Content

{ "type": "https://trackline.local/errors/quota-exceeded",
  "title": "Device quota exceeded", "status": 422,
  "detail": "Tier 'free' permits 50 devices; 50 are in use.",
  "traceId": "4bf92f…" }
```

`422`, not `429` — retrying will never help. The caller must upgrade or delete something.

### Telemetry backpressure

Ingest does **not** return an error for pacing. It sets `retry_after_ms` in
`StreamTelemetryResponse`, and well-behaved devices slow down. Only devices that ignore
it repeatedly are throttled outright — otherwise a transient slowdown becomes a
thundering herd of reconnects.

---

## 6. Load shedding

Rate limits are per tenant. Shedding is global, and it activates when the system cannot
keep up regardless of who is asking.

### Signals

| Signal | Threshold |
|---|---|
| Ingest queue depth | > 80% of buffer capacity |
| Kafka producer latency | p99 > 500 ms |
| Downstream error rate | > 10% over 30 s |
| Consumer lag | > 5 minutes |

### Order of sacrifice

Shed in this order. Each level is fully exhausted before the next begins:

1. **Reduce sampling for Free tier** — accept 1 in N points, keep the connection alive
2. **Reject Free tier telemetry** with `retry_after_ms`
3. **Reduce sampling for Pro tier**
4. **Reject Pro tier telemetry**
5. **Degrade features for everyone** — disable history queries, serve the map from cache
6. **Enterprise telemetry is never shed**

Writes are preserved over reads. A customer losing their audit trail is worse than a
customer's dashboard being slow.

### What is never shed

- Authentication — shedding auth makes the system undebuggable during an incident
- Health and readiness probes — shedding these gets you killed by Kubernetes
- Billing events — `usage.recorded` losses are unrecoverable revenue
- Critical alerts already raised

---

## 7. Fairness

Per-tenant token buckets are necessary but not sufficient. A tenant within its limit can
still monopolise a shared resource — a connection pool, a Kafka partition, a worker.

| Resource | Protection |
|---|---|
| DB connection pool | Per-tenant concurrent-query cap (⌈pool/4⌉) |
| Ingest workers | Weighted fair queueing by tier |
| Notification workers | Per-tenant in-flight cap |
| Kafka partitions | Keyed by vehicle, not tenant — see `kafka-topics.md` |

This is the noisy-neighbour problem, and it is why the seed data makes tenants unequal.
With equal tenants you can never demonstrate that any of this works.

---

## 8. Observability requirements

The application emits; the platform collects and alerts.

| Metric | Type | Labels |
|---|---|---|
| `trackline_ratelimit_decisions_total` | counter | `tier`, `decision`, `route_class` |
| `trackline_quota_rejections_total` | counter | `tier`, `quota_type` |
| `trackline_shed_points_total` | counter | `tier`, `shed_level` |
| `trackline_shed_level` | gauge | — |
| `trackline_tenant_inflight` | gauge | `tier` |

**Label with `tier`, never with `tenant_id`.** Tenant id is unbounded cardinality and will
destroy Prometheus. Per-tenant breakdown belongs in logs and traces, which are designed for
high cardinality — metrics are not.

---

## 9. Alerting

| Condition | Severity |
|---|---|
| Shed level > 0 for more than 60 s | page |
| Rate limiter failing open | page |
| Any tenant at > 90% of device quota | ticket |
| Quota rejections spiking for one tenant | ticket |
| Free tier shedding during business hours | ticket |

---

## 10. Open questions

| Question | Decide by |
|---|---|
| Does exceeding device quota block ingest from existing devices, or only new registrations? | Phase 1 |
| Should Enterprise be able to purchase burst capacity? | Phase 6 (FinOps) |
| Is shedding per-region or global in a multi-region deployment? | Phase 5 |
| Should rate limits be per-principal as well as per-tenant? | Phase 2 |
