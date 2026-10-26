# Code Annotations — Why Things Are Written This Way

> A companion to the six Phase 0 services in `apps/`. Every file below has real
> comments in the source explaining a non-obvious decision — this document
> collects them in one place so you can review the reasoning without hunting
> through six codebases in three languages. Visit this when something in the
> code looks arbitrary; it almost never is.
>
> Format: **file → the comment, in context → why it matters.**

---

## edge-gateway (Go)

### `internal/auth/context.go` — the header-stripping rule

> _"The rule that matters most in this whole service... every inbound
> `X-Trackline-_`header is stripped before the gateway sets its own. Without
that, a client can forge`X-Trackline-Tenant` and read another tenant's data
> once the request reaches fleet-api. That is a confused-deputy attack and the
> most likely way this system gets breached."\*

This is the single most important line of reasoning in the entire codebase.
`edge-gateway` is the _only_ trust boundary (`apps/_contracts/auth.md` §3) —
every service behind it trusts headers it didn't set itself. Get this wrong and
tenant isolation doesn't exist, no matter how correct `fleet-api`'s queries are.

### `internal/auth/jwt.go` — never trust an unresolved JWKS

> _"A JWKS fetch failure must never cause tokens to be accepted unvalidated."_

The validator's `keyFor` function returns an error rather than falling back to
a stale-but-unverified key when the cache is empty and a refresh fails. The
tempting shortcut — "just accept it, JWKS is probably still fine" — is exactly
the bug this comment exists to prevent.

### `internal/ratelimit/bucket.go` — fail open, deliberately

> _"If Redis is unreachable, this limiter fails OPEN (serves the request).
> Rejecting all traffic because the limiter itself is down converts a
> degradation into an outage."_

This is a conscious availability-over-fairness trade, not an oversight. The
`Decision.FailedOpen` flag exists specifically so this condition can be
alerted on — a silently-disabled rate limiter is how one tenant takes down
everyone else (`apps/_contracts/limits.md` §4).

### `main.go` — middleware order is load-bearing

> _"Reordering this chain is a security bug."_

`withMiddleware` runs strip → authenticate → rate-limit → assert-context →
proxy, in that exact order. Swapping steps 1 and 4 reintroduces the
confused-deputy vulnerability above.

---

## ingest (Go)

### `internal/grpcserver/telemetry.go` — statelessness is the point

> _"This is the hot path: every field touched here is multiplied by device
> count times emission rate."_

Any temptation to add a convenience field to `TelemetryPoint` should be
weighed against this. See also `apps/_proto/trackline/v1/telemetry.proto`'s
matching comment on the message itself.

> _"StreamTelemetry... never returns a hard error for pacing; it sets
> `retry_after_ms` and lets the caller decide... Only devices that ignore it
> repeatedly get throttled outright, which is a decision made above this
> layer."_

This is why `ingest` never rejects a well-behaved device outright for going
too fast — that would create a thundering herd of reconnecting devices, which
is worse than the original overload (`apps/_contracts/limits.md` §5).

### `internal/kafka/producer.go` — why `acks=all` and keyed writes

> _"acks=all and an idempotent producer, so a lost telemetry point is never
> silent and a retried publish is never duplicated on the broker side."_

Paired with `apps/_contracts/kafka-topics.md` §2's explanation of _why_
`vehicle_id` and not `tenant_id` is the partition key (ordering vs. hot
partitions — your unequal tenant sizes would turn a tenant-keyed topic into a
single overloaded partition).

### `README.md` — dedup is in-memory by default, Redis-backed on request

> _"`PublishBatch`'s `batch_id` is de-duplicated ... in memory by default, and in
> Redis when `REDIS_ADDR` is set."_

The original gap (a restart forgets every batch id, a second replica never knew them)
is closed by the shared Redis tier, but only when it is configured: without
`REDIS_ADDR` the in-memory behaviour and its limits are unchanged, and a Redis outage
falls back to memory rather than refusing batches.

---

## fleet-api (NestJS)

### `common/tenant-guard.ts` — 404, never 403

> _"Cross-tenant access returns 404, never 403. A 403 confirms the resource
> exists, which is an information leak."_

Achieved almost for free: every query's `WHERE tenant_id = $1 AND id = $2`
clause makes another tenant's row indistinguishable from a nonexistent one.
See the identical pattern repeated in `fleets/fleets.service.ts`'s
`getOrThrow` methods — that repetition is intentional, not copy-paste laziness.

### `fleets/fleets.service.ts` — why the quota check runs inside a transaction

> _"Runs inside the caller's transaction so the count-then-insert is atomic
> under concurrent requests — the whole reason this lives in fleet-api and not
> at the edge."_

`edge-gateway`'s rate limiter answers "too fast right now?"; this answers "are
you entitled to this device at all?" (`apps/_contracts/limits.md` §1). Only a
real database transaction can make that count-then-insert race-free, which is
why the check couldn't have lived in the gateway.

### `common/db.module.ts` — the pool is small on purpose

> _"Deliberately small in Phase 0. `limits.md` §7 caps per-tenant concurrent
> queries at `ceil(pool/4)` — a pool this small makes that cap bite quickly
> and visibly during load testing, rather than hiding the problem until Phase
> 6."_

If this number ever looks "too small to be practical," that's the point —
it's meant to surface pool-exhaustion behaviour early, not to be tuned away
before you've seen it happen once.

### `dev-auth/dev-auth.module.ts` — a scaffold with an expiry date

> _"This entire file is deleted the moment Dex/Keycloak (Phase 1) or Entra ID
> (Phase 5) exists — do not build on top of it."_

Also: the module refuses to fabricate its own signing key silently — it
throws if `.dev-keys/private.pem` is missing, because a key generated
implicitly and never rotated is exactly the anti-pattern
`apps/_contracts/auth.md` warns against.

---

## notifier (Rust)

### `src/retry.rs` — jitter is not decoration

> _"Jitter is not optional decoration. Without it, every failed delivery for
> the same outage retries in lockstep, producing a synchronized retry storm
> the moment the downstream recovers."_

`backoff_for_attempt` adds up to 30% random jitter on top of exponential
backoff specifically to break that synchronization. This is the concrete,
working example of the "retry storm" failure mode listed in
`docs/05_STUDY_GUIDE.md` Phase 6.

### `src/dlq.rs` — never block a partition on one bad message

> _"One unparseable notification must not stall delivery for every other
> alert on the same partition."_

This is why malformed payloads go straight to the DLQ with zero retries in
`main.rs`, rather than being retried in place — retrying in place is exactly
what _would_ stall the partition.

### `src/channels/webhook.rs` — the signature scheme is the contract

> _"Signs body as `"{t}.{raw_body}"` keyed by secret — the exact scheme
> documented in auth.md §7, so a customer's receiver implementation can be
> verified against that doc alone, without reading this code."_

If you ever change the signing scheme here, `apps/_contracts/auth.md` §7 must
change in the same commit — they are one contract split across two files.

### `src/channels/email.rs` — why Mailpit, always

> _"No development code path may be configured to reach a real inbox —
> protects the lab domain's sending reputation before it even exists."_

Directly implements the reasoning in `docs/04_ENVIRONMENTS.md` §9 about why a
lab domain's sending reputation is fragile and worth protecting proactively.

### `main.rs` — commit after processing, not before

> _"Committing early is how at-least-once quietly becomes at-most-once and
> messages get lost on a crash."_

`enable.auto.commit` is `false` and the offset commit happens only after
`deliver_with_retry` returns — a crash between receive and commit means the
message is redelivered, which is what the idempotency-key check earlier in the
loop is there to absorb.

---

## device-sim (Go)

### `internal/profiles/profiles.go` — why four profiles, not one

> _"Uniform simulated devices hide the load patterns real, mixed fleets
> actually produce."_

This is the direct mechanism behind `docs/01_APPLICATION.md`'s point about
unequal tenants — a fleet of identical, well-behaved devices can never
reproduce a hot partition, a noisy-neighbour effect, or a false-positive alert
rate. The `erratic` profile exists specifically to stress
`ingest/internal/validate/validate.go`.

### `cmd/simulator/main.go` — one device's failure is not the fleet's failure

> _"A single vehicle's connection dropping must never take down the rest of
> the simulated fleet."_

Each simulated vehicle runs its own goroutine with its own reconnect loop —
mirroring how you'd want a real device fleet to behave, and giving you a
realistic "some devices are flaky" condition to test against rather than an
all-or-nothing connection.

---

## web (Next.js)

### `lib/api-client.ts` — one door in, not many

> _"Do not add ad-hoc `fetch` calls elsewhere in the app; extend this file so
> every request goes through one place that handles the auth header and error
> shape consistently."_

This is a discipline note as much as a technical one. The moment a second
`fetch` call appears somewhere else in the app, the auth-header and
`Problem`-parsing logic has to be duplicated or drift.

### `lib/ws-client.ts` — the gap is documented, not hidden

> _"The `/v1/positions/stream` endpoint this connects to does not exist in
> fleet-api yet... Adding the WebSocket upgrade path... is unfinished
> application work — track it as a follow-up story before this file is relied
> upon."_

The map page (`app/map/page.tsx`) degrades gracefully and visibly reports
"REST snapshot only" rather than failing silently — a deliberate choice about
how to handle a known-incomplete dependency.

---

## Reading this file usefully

- If you're debugging a cross-tenant data leak, start with the `edge-gateway`
  and `tenant-guard.ts` entries above.
- If you're debugging a retry storm or duplicate side effects, start with
  `notifier`'s three entries.
- If a load test doesn't reproduce a real-world failure, check whether
  `device-sim`'s profile mix is actually varied — a uniform fleet is the
  usual reason.
- Every comment quoted here also exists verbatim in the source. If this
  document and the code ever disagree, the code is correct and this file is
  stale — update it in the same PR that changes the reasoning.
