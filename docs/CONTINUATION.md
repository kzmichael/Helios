# HELIOS Continuation

Last updated: 2026-10-27

This is the short-lived working handoff for resuming HELIOS across sessions and machines. `AGENTS.md` explains how agents must use and update it. Verify relevant claims against the current checkout before acting; do not treat this note as a substitute for tests or acceptance evidence.

## Current focus

Phase 0, local application foundation. The user reports that containerization is complete and wants to continue the project rather than repeat that work.

On 2026-10-27 the assistant implemented the remaining application work under `apps/` (commit `3c96594 Added all codes`). The operator-owned SRE work was deliberately left untouched. Current Git state is `feature/Initial-setup` at `762b0f7 Updated Agent`; the worktree is clean, and the cached `github/feature/Initial-setup` and `azure/feature/Initial-setup` refs match HEAD. The terminal records `git push -u azure` succeeding. The next step is to validate the operator-owned local stack against the application.

## Recent verified work

- `.mise.toml` pins the project toolchain; `mise ls` showed the pinned tools installed in this checkout.
- Gitleaks scanned four commits on 2026-10-14 and reported no leaks.
- `buf generate` succeeded. `buf lint` failed because `TelemetryIngest` in `apps/_proto/trackline/v1/telemetry.proto` does not meet the configured service-name suffix rule.
- `go test ./...` compiled `edge-gateway`, `ingest`, and `device-sim` after protobuf generation. The packages reported no test files.
- On 2026-10-27, Docker was available and `docker compose config --quiet` passed for the current Compose file. This checks configuration syntax only; no Compose services were started and no application integration was verified.

## Application work completed 2026-10-27 (assistant)

What changed, per service. Detail lives in each service README under `apps/`.

- `device-sim`: plays the seeded 500-device fleet (150 seeded + 350 from the opt-in `apps/_db/load/load-fleet.sql`), scripted scenarios (`SCENARIO=speeding|offline|port-entry|hub-exit`), seeded chaos injection.
- `ingest`: opt-in device registry, device auth (`DEVICE_AUTH=required`), per-device rate limit, tier-aware shedding (`LOAD_SHEDDING=on`), Redis batch dedup, MQTT transport. All gated by env; defaults keep the old behaviour.
- `apps/pipeline` (new Go binary, roles chosen by `ROLES`): registry-sync, position-updater, telemetry-archiver, detector (Phase 0 stand-in for Flink), alert-engine, delivery-recorder, usage-collector.
- `edge-gateway`: API-key auth, `?token=` for `/v1/positions/stream` only, 1 MB body cap, problem+json `429` with `X-RateLimit-*`.
- `fleet-api`: every OpenAPI path plus `WS /v1/positions/stream`, class-validator DTOs, problem+json errors, role guard, `audit_log` writes, `POST /internal/api-keys/verify` (internal only), page size capped at 200, numeric columns returned as numbers.
- `notifier`: protobuf and JSON decode, SMS channel, publishes `notifications.delivered`; `WEBHOOK_SECRET` is needed for webhooks (otherwise dead-lettered).
- `web`: Leaflet live map, `/vehicles`, `/vehicles/[id]` history, `lib/api-types.ts` generated from the contract (`pnpm gen:api`).
- Phase 2: `stream-proc` (Flink 1.20, Java 17), `billing-batch` (Spring Batch, `--period=YYYY-MM`, exit 0/2/64), `eta-ml` (FastAPI + `python -m eta_ml.train`).
- Contracts/DB: migration `V3` (unique index on `notification_delivery.idempotency_key`; the pipeline needs it), `kafka-topics.md` section 9, `auth.md` API-key and WebSocket sections, `/positions/stream` in `openapi.yaml`.
- Go modules are pinned to `go 1.23` with older dependency versions so they build on the `golang:1.25.12` images; a newer `go get` silently raises that.

### Verified by the assistant (throwaway Kafka/Postgres/Redis, since removed)

- Unit tests pass: ingest, pipeline, edge-gateway, device-sim (Go); fleet-api (jest); stream-proc (13), billing-batch, eta-ml (16); notifier (8, built in a container).
- End to end: 500 devices through `ingest` with auth required, `pipeline`, `fleet-api`, the gateway (JWT, API key, live WebSocket) and `web` (checked in a browser). `stream-proc` events became alerts via `alert-engine`. `billing-batch` reran idempotently and failed with exit 2 on a corrupted invoice. `eta-ml` trained and served from simulated telemetry.
- The existing Dockerfiles for ingest, edge-gateway, device-sim, fleet-api and web built successfully against the changed code.

### Not verified

- MQTT was never run against a broker.
- Notifier: emails reached Mailpit; the full webhook/SMS to `delivered` to `delivery-recorder` loop was not re-run.
- No images or Compose services exist for `pipeline`, `stream-proc`, `billing-batch`, `eta-ml`; nothing was run under the user's Compose stack.
- Flink was only run standalone, not on a cluster.

## Containerization status

- User-reported complete: containerization of all six Phase 0 services. Do not restart image work by default; resume it only if the user asks or a later integration check exposes a concrete problem.
- The checkout contains six service Dockerfiles and `docs/adr/0004-container-base-image-strategy.md`, which records the web image size exception.
- The current `docker-compose.yml` contains Redis, PostgreSQL, Kafka, Flyway, the six Phase 0 service images, Mailpit, and a webhook sink. `docker compose config --quiet` passed on 2026-10-27, but the services were not started; health, persistence, data flow, and load acceptance remain unverified.
- `Taskfile.yml` and `docker-compose.obs.yml` were empty on the latest check. `scripts/verify-image-standard.sh` was previously found to be a stub; it has not been rechecked in this update. Do not claim these acceptance checks pass based only on image files or the user's report.
- Docker is available in the current Windows environment. The earlier note that it was unavailable applied to a prior check/session and is no longer current.

## Next task

Operator work: validate the local data layer and then wire/test the completed application, following the Phase 0 order in `docs/03_ROADMAP.md` and `backlog/phase-0.yaml`. The Compose file parses, but `HEL-0.4.1` is not accepted until cold-start health, Kafka KRaft mode, persistence across restart, volume reset, and CLI connectivity are verified. Then complete `HEL-0.4.2` migration/seed verification before treating `HEL-0.4.3` and `HEL-0.4.4` as ready for acceptance. `Taskfile.yml` remains empty, so the one-command workflow and clean-rebuild criteria are still open.

Wiring the stack needs, at minimum (each service README lists its variables):

- A `pipeline` service (new Dockerfile, build context `apps/`) with `KAFKA_BROKERS`, `DATABASE_URL`, `REDIS_ADDR`; and `REDIS_ADDR` on `fleet-api` and `ingest`.
- Apply migration `V3`; apply `apps/_db/load/load-fleet.sql` for the 500-device fleet. All contract Kafka topics must exist (auto-create is off by design).
- `WEBHOOK_SECRET` and an SMS sink (`SMS_GATEWAY_URL`) on `notifier`; `CORS_ALLOWED_ORIGINS` on the gateway; the gateway must reach `fleet-api` (API-key verification); `/internal/*` on `fleet-api` must not be exposed.
- Acceptance evidence to collect: device-sim at 500 devices shows moving markers on `/map`, `/vehicles/{id}` shows history, a scenario produces an alert and a Mailpit message, `usage_daily` fills.

Phase 2 deployables (`stream-proc`, `billing-batch`, `eta-ml`) need Dockerfiles and runtime design (checkpoint storage, CronJob policy, model artifact storage); they are not Phase 0 blockers. The container-image verifier discrepancy above remains visible; do not quietly mark its acceptance criteria as verified.

## Other open discrepancies

- The committed `.githooks/pre-commit` exists, but `core.hooksPath` was unset in the last check; the hook was therefore not confirmed active.
- `README.md`, the roadmap baseline/next-action section, and `docs/SCAFFOLD_STATUS.md` contain stale status claims. Refresh local progress documentation when acceptance evidence is available. Do not update ADO unless explicitly requested.
- No Phase 0 end-to-end stack validation under the user's Compose stack has been done by the assistant; the 2026-10-27 application tests used a throwaway environment, not Compose.
- The worktree was clean at `762b0f7` on 2026-10-27, and cached GitHub/Azure branch refs matched. The terminal records a successful Azure push. These remote comparisons use cached refs, not a fresh fetch.
- Do not update ADO unless asked.
