# edge-gateway

Public entrypoint for Trackline. Authenticates every request, enforces per-tenant
rate limits, then proxies to `fleet-api`. Implements `apps/_contracts/auth.md` and
`apps/_contracts/limits.md` §4.

---

## What this service does, and does not, do

| Does                                                                                | Does not                                                                   |
| ----------------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| Validate JWTs against a JWKS endpoint                                               | Issue tokens — that's the identity provider's job                          |
| Authenticate machine clients by API key (`X-Trackline-Key`)                         | Store or see an API key secret — it hashes the secret and asks `fleet-api` |
| Strip and re-assert `X-Trackline-*` trust headers                                   | Trust any inbound `X-Trackline-*` header, ever                             |
| Rate-limit per tenant per tier; answer `429` with `Retry-After` and `X-RateLimit-*` | Enforce device/vehicle quotas — that's `fleet-api`                         |
| Cap request bodies at 1 MB (`limits.md` §3)                                         | Handle telemetry — devices talk to `ingest` directly over gRPC/MQTT        |
| Proxy REST and the positions WebSocket to `fleet-api`                               |                                                                            |

## Why the middleware order is fixed

`withMiddleware` (in `cmd/gateway/middleware.go`) runs, in this exact order: strip
inbound trust headers → authenticate → rate limit → assert trusted context → proxy.
Swapping steps 1 and 4 is the confused-deputy vulnerability described in `auth.md`
§3 — a client could set `X-Trackline-Tenant` itself and read another tenant's data.

One subtlety the order creates: the API key travels in an `X-Trackline-*` header,
which step 1 deletes. The key is therefore read _before_ the strip, used only to
authenticate, and never forwarded. A test pins both halves.

## Credentials

| Principal         | Presented as                                      | Checked by                                                                                                                                                                                                                                                                               |
| ----------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Human             | `Authorization: Bearer <JWT>`                     | JWKS validation here                                                                                                                                                                                                                                                                     |
| Machine client    | `X-Trackline-Key: <prefix>.<secret>`              | `POST /internal/api-keys/verify` on `fleet-api` with `{prefix, sha256(secret)}` — the raw secret never leaves this process. Results are cached 30 s (5 s for rejections), so revocation is eventually consistent. If `fleet-api` cannot be reached the answer is `503`, never "allowed". |
| Browser WebSocket | `?token=<JWT>` on **`/v1/positions/stream` only** | Same JWT validation. A browser cannot set `Authorization` on a WebSocket. The parameter is removed before forwarding so the token never reaches `fleet-api`'s logs; on every other path a query token authenticates nothing.                                                             |

Errors are `application/problem+json`. A rate-limited reply follows `limits.md` §5:
`429`, `Retry-After`, `X-RateLimit-Limit` / `-Remaining` / `-Reset`.

## Environment variables

| Variable               | Default                                              | Meaning                                                        |
| ---------------------- | ---------------------------------------------------- | -------------------------------------------------------------- |
| `LISTEN_ADDR`          | `:8080`                                              | Where this service listens                                     |
| `FLEET_API_URL`        | `http://fleet-api:3000`                              | Upstream for all `/v1/*` traffic, and for API-key verification |
| `CORS_ALLOWED_ORIGINS` | —                                                    | Comma-separated browser origins allowed to call the API        |
| `OIDC_ISSUER`          | `https://auth.trackline.local/`                      | Expected `iss` claim                                           |
| `JWKS_URL`             | `https://auth.trackline.local/.well-known/jwks.json` | Where to fetch signing keys                                    |
| `EXPECTED_AUD`         | `trackline-api`                                      | Expected `aud` claim                                           |
| `REDIS_ADDR`           | `redis:6379`                                         | Backs the rate limiter                                         |

In Phase 0 there is no real IdP yet (`auth.md` §1 table) — `fleet-api` mints
development tokens signed with a local key pair, and `JWKS_URL` points at whatever
serves that key pair locally.

## Running it

```bash
cd apps/edge-gateway
go test ./...
go run ./cmd/gateway
```

The module is pinned to `go 1.23`. The tests cover the header-stripping rule, API-key
authentication (and that the key is not forwarded), the verifier outage path, the
`429` shape, the body cap and the WebSocket token rule. End to end against a real
`fleet-api` and Redis: a bearer token, an API key (good and bad) and a live
WebSocket stream all worked through the gateway.

## What's deliberately not implemented yet

- Load shedding (`limits.md` §6 level 5 — serve the map from cache, disable history
  queries): arrives with Phase 6 load testing, where its thresholds can be tuned
  against measurements
- OpenTelemetry instrumentation — arrives in Phase 3, on purpose (see
  `docs/07_REPOSITORY_BLUEPRINT.md` §3.3)
