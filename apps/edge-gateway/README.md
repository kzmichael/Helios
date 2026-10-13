# edge-gateway

Public entrypoint for Trackline. Authenticates every request, enforces per-tenant
rate limits, then proxies to `fleet-api`. Implements `apps/_contracts/auth.md` and
`apps/_contracts/limits.md` §4.

---

## What this service does, and does not, do

| Does | Does not |
|---|---|
| Validate JWTs against a JWKS endpoint | Issue tokens — that's the identity provider's job |
| Strip and re-assert `X-Trackline-*` trust headers | Trust any inbound `X-Trackline-*` header, ever |
| Rate-limit per tenant per tier | Enforce device/vehicle quotas — that's `fleet-api` |
| Proxy REST traffic to `fleet-api` | Handle telemetry — devices talk to `ingest` directly over gRPC/MQTT |

## Why the middleware order in `main.go` is fixed

`withMiddleware` runs, in this exact order: strip inbound trust headers →
authenticate → rate limit → assert trusted context → proxy. Swapping steps 1 and 4
is the confused-deputy vulnerability described in `auth.md` §3 — a client could set
`X-Trackline-Tenant` itself and read another tenant's data.

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Where this service listens |
| `FLEET_API_URL` | `http://fleet-api:3000` | Upstream for all `/v1/*` traffic |
| `OIDC_ISSUER` | `https://auth.trackline.local/` | Expected `iss` claim |
| `JWKS_URL` | `https://auth.trackline.local/.well-known/jwks.json` | Where to fetch signing keys |
| `EXPECTED_AUD` | `trackline-api` | Expected `aud` claim |
| `REDIS_ADDR` | `redis:6379` | Backs the rate limiter |

In Phase 0 there is no real IdP yet (`auth.md` §1 table) — `fleet-api` mints
development tokens signed with a local key pair, and `JWKS_URL` points at whatever
serves that key pair locally.

## Running it

```bash
cd apps/edge-gateway
go mod tidy   # resolves golang-jwt and go-redis; needs network access
go run ./cmd/gateway
```

**Not yet run.** There is no Go toolchain confirmed on the target machine yet
(`HEL-0.1.2`), and this depends on Redis and `fleet-api` being up. Building it is
part of `HEL-0.3.1`/`HEL-0.3.2` (containerise and verify), and wiring it into the
compose stack is `HEL-0.4.3`.

## What's deliberately not implemented yet

- API key authentication (`X-Trackline-Key`) — only bearer JWTs are validated
- Load shedding (`limits.md` §6) — arrives with Phase 6 load testing
- OpenTelemetry instrumentation — arrives in Phase 3, on purpose (see
  `docs/07_REPOSITORY_BLUEPRINT.md` §3.3)
