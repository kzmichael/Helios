# fleet-api

Tenants, fleets, vehicles, drivers, geofences, alerts. The boring-but-essential
core of Trackline — implements `apps/_contracts/openapi.yaml`. This is where
database problems live: slow queries, connection pool exhaustion, migrations.

## Module map

| Module                   | Owns                                                           | Implements                                       |
| ------------------------ | -------------------------------------------------------------- | ------------------------------------------------ |
| `common/db.module.ts`    | Postgres pool                                                  | —                                                |
| `common/tenant-guard.ts` | Reading gateway-asserted tenant context                        | `auth.md` §3–4 (404-not-403 isolation)           |
| `common/http.ts`         | Role guard (`@MinRole`), id-param pipe, paging DTO             | `auth.md` §4 roles; `limits.md` §3 page-size cap |
| `common/problem.ts`      | Validation pipe and `application/problem+json` for every error | `openapi.yaml` `Problem`                         |
| `common/pg-types.ts`     | Parses `bigint`/`numeric` as JSON numbers                      | `openapi.yaml` numeric fields                    |
| `common/audit.ts`        | `@Audited` interceptor writing `audit_log`                     | `auth.md` §8                                     |
| `tenants/`               | Device quota enforcement                                       | `limits.md` §3                                   |
| `fleets/`                | Fleet + vehicle CRUD, vehicle filters, position history        | `/fleets`, `/vehicles`, `/vehicles/{id}/history` |
| `geofences/`             | Geofence create/list, polygon validation, derived radius       | `/geofences`                                     |
| `alerts/`                | Alert list/acknowledge                                         | `/alerts`                                        |
| `positions/`             | Last-known positions and the live WebSocket                    | `/positions`, `/positions/stream`                |
| `drivers/`               | Driver list                                                    | `/drivers`                                       |
| `billing/`               | Usage and invoices (owner role only)                           | `/usage`, `/invoices`                            |
| `internal/`              | API-key verification for `edge-gateway`                        | not public; see below                            |
| `dev-auth/`              | **Phase 0 only** — mints dev JWTs, serves a JWKS               | Stand-in until Dex/Keycloak (Phase 1)            |

## Behaviour worth knowing

- **Audit trail** (`auth.md` §8): every successful create, update, delete and
  acknowledge writes an `audit_log` row — actor kind (`user`, or `service` for an API
  key), actor id from the gateway-asserted subject, action, target and the request's
  trace id. Rejected requests change nothing and write nothing; credentials are never
  recorded. If the audit insert itself fails the operation has already committed, so
  the failure is logged loudly rather than turned into an error response.
- **Roles** (`auth.md` §4): reads need nothing beyond a tenant context; creating,
  updating, deleting and acknowledging need `operator`; usage and invoices need
  `owner`. A role that is too low is `403`; another tenant's row is always `404`.
- **Validation** is class-validator on every body and query, limits taken from
  `openapi.yaml`; failures are `400` problem documents naming each field. A path id
  that is not a UUID is `404`, the same answer as an id in another tenant. Page size
  is capped at 200.
- **Ids in request bodies are checked too**: a `fleetId` naming another tenant's fleet
  is `404`, not stored.
- **History** (`/vehicles/{id}/history`) is oldest-first and clipped to the tenant's
  retention (7 / 90 / 730 days), so asking for more than the tier keeps returns what it
  does keep.
- **Deleting a fleet** with vehicles in it is `409`.
- **Geofences** get a real radius (the smallest circle around the centroid that
  contains the polygon) instead of a placeholder.
- **Live positions.** `GET /positions` is the first paint, from
  `vehicle_position_latest`; `WS /positions/stream` pushes every position of the
  tenant as it arrives, fed by Redis `positions:{tenantId}`, which the pipeline's
  `position-updater` publishes to. A slow reader loses frames rather than growing this
  process's memory. Redis being down degrades the stream, not the service.
- **`POST /internal/api-keys/verify`** is called only by `edge-gateway`. It takes
  `{prefix, secretHash}`, answers the key's tenant, tier and roles — or a bare `401`
  that does not say whether the key was unknown, wrong, revoked, expired or its tenant
  suspended. It is **not** part of the public API and must not be reachable from
  outside the cluster (`auth.md` §3); the gateway proxies only `/v1/*`.

## Why `dev-auth/` exists and why it must not survive Phase 1

There is no real identity provider in Phase 0 (`auth.md` §1). Something has to
sign tokens edge-gateway can validate, so `fleet-api` mints them, using a local
RS256 key pair that **you generate**, not one the code fabricates silently. The
module is excluded automatically when `NODE_ENV=production`. Delete it outright
once Dex or Keycloak exists — do not extend it.

### Generating the dev key pair (you do this, not me)

```bash
bash scripts/dev-token-keys.sh
```

`.dev-keys/` must be in `.gitignore`. It is a private key, in a public repo.
The script preserves an existing private key and regenerates its matching JWKS.

## Environment variables

| Variable                 | Default      | Meaning                                                                                 |
| ------------------------ | ------------ | --------------------------------------------------------------------------------------- |
| `PORT`                   | `3000`       | HTTP listen port                                                                        |
| `DATABASE_URL`           | —            | Postgres connection string                                                              |
| `PG_POOL_MAX`            | `10`         | Deliberately small — makes pool exhaustion visible early                                |
| `REDIS_ADDR`             | `redis:6379` | Subscribed to for the live positions stream; unreachable Redis degrades only that       |
| `POSITION_STALE_SECONDS` | `300`        | A position older than this is flagged `stale`                                           |
| `NODE_ENV`               | —            | Only `production` disables `dev-auth/`; Phase 0 local auth needs a non-production value |

## Container runtime contract (Phase 0)

This describes what the application needs from an image and its local runtime.
The operator owns the Dockerfile and Compose configuration.

- **Toolchain and dependencies:** use Node 22 and pnpm as pinned in `.mise.toml`.
  `pnpm build` invokes the Nest CLI, which is a dev dependency; install build
  dependencies in the build stage. The runtime needs the compiled application
  and production dependencies, not the Nest CLI. Use `pnpm-lock.yaml` for
  reproducible installs; a separate `package-lock.json` is also present and
  must not be mixed into the pnpm image workflow. With the pinned pnpm 12,
  `pnpm install --frozen-lockfile` currently stops because the `@nestjs/core`
  postinstall script has not been explicitly approved or denied. Settle that
  dependency build-script policy before claiming a clean install works.
- **Entrypoint and port:** `tsconfig.json` emits CommonJS files from `src/` into
  `dist/`; `pnpm build` now produces `dist/main.js`. Run it with
  `node dist/main.js` from the service root, listening on `PORT` (default
  `3000`). This has been built and run directly with Node against Postgres and Redis,
  but not yet in a container; `pnpm test` runs the jest specs (roles, id and paging
  validation, problem responses). The `ws` and `ioredis` packages are runtime
  dependencies (the live stream). Every endpoint was exercised end to end through
  `edge-gateway`. `pnpm start`
  invokes the Nest CLI and is not the intended minimal-image entrypoint.
- **Dependencies and health:** configure `DATABASE_URL` at runtime for
  database-backed requests; `PG_POOL_MAX` defaults to 10. `/healthz` responds
  once the HTTP server is running but does not check PostgreSQL, so it is a
  liveness signal, not proof that the database is ready.
- **Local development identity:** Phase 0 needs dev tokens and JWKS from this
  service. Run only in a local environment with `NODE_ENV` other than
  `production`. The dev-auth module reads `.dev-keys/private.pem` and
  `.dev-keys/jwks.json` relative to the service root (two levels above the
  compiled `dist/dev-auth/` directory). Generate them from the repository root
  with `bash scripts/dev-token-keys.sh`; never copy keys into an image or commit
  them.
  `NODE_ENV=production` disables dev-auth and its endpoints, so it is not a
  substitute for a production-dependency-only image in the Phase 0 stack.
- **Shutdown:** the application currently registers no explicit Nest shutdown
  hook or connection-drain procedure. Do not claim graceful SIGTERM or clean
  database-pool shutdown based on a successful build; test the running
  container and report any missing behavior to the app team.

## Running it

```bash
cd apps/fleet-api
pnpm install
pnpm start:dev
```

**Not yet verified running.** A pinned Node toolchain is present in `.mise.toml`,
and `pnpm build` succeeds after adding `@types/express` as a dev dependency.
Database-backed startup has not been verified.
`drivers/`, contract-generated `web` client
types, and full request validation (`class-validator`) are intentionally absent
from this first pass — add them as you extend the module, not before.
