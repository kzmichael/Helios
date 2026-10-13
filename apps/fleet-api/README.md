# fleet-api

Tenants, fleets, vehicles, drivers, geofences, alerts. The boring-but-essential
core of Trackline — implements `apps/_contracts/openapi.yaml`. This is where
database problems live: slow queries, connection pool exhaustion, migrations.

## Module map

| Module | Owns | Implements |
|---|---|---|
| `common/db.module.ts` | Postgres pool | — |
| `common/tenant-guard.ts` | Reading gateway-asserted tenant context | `auth.md` §3–4 (404-not-403 isolation) |
| `tenants/` | Device quota enforcement | `limits.md` §3 |
| `fleets/` | Fleet + vehicle CRUD | `openapi.yaml` `/fleets`, `/vehicles` |
| `geofences/` | Geofence CRUD + polygon validation | `openapi.yaml` `/geofences` |
| `alerts/` | Alert list/acknowledge | `openapi.yaml` `/alerts` |
| `dev-auth/` | **Phase 0 only** — mints dev JWTs, serves a JWKS | Stand-in until Dex/Keycloak (Phase 1) |

Not yet built: `drivers/`, `/positions`, `/vehicles/{id}/history`, `/usage`,
`/invoices`. Same patterns as `fleets/` — add them following that module as the
template.

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

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `3000` | HTTP listen port |
| `DATABASE_URL` | — | Postgres connection string |
| `PG_POOL_MAX` | `10` | Deliberately small — makes pool exhaustion visible early |
| `NODE_ENV` | — | Only `production` disables `dev-auth/`; Phase 0 local auth needs a non-production value |

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
  `3000`). This has been compiled but not started in a container. `pnpm start`
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
