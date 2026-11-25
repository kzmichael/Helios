# HELIOS Continuation

Last updated: 2026-11-25 (macOS checkout, after the Windows update earlier the same day).

This is the short-lived working handoff for resuming HELIOS across sessions and machines. `AGENTS.md` explains how agents must use and update it. Verify relevant claims against the current checkout before acting; do not treat this note as a substitute for tests or acceptance evidence.

## Current focus

Phase 0, local application foundation. The user reports that containerization is complete and wants to continue the project rather than repeat that work.

On 2026-10-27 the assistant implemented the remaining application work under `apps/` (commit `3c96594 Added all codes`). The operator-owned SRE work was deliberately left untouched. The next step is to validate the operator-owned local stack against the application.

Git: work is on `feature/Initial-setup`; verify the current commit with `git log` and `git status`. Remotes differ by machine: the Windows checkout has `github` (`https://github.com/kzmichael/Helios.git`) plus `azure` and `origin`, which both point to the Azure DevOps repo; the macOS checkout reported only `origin`, pointing at GitHub. Check `git remote -v` on each machine before pushing. `agent update in` pulls and reads this file; `agent update out` rewrites it, commits, and pushes to both remotes (see `AGENTS.md`).

Since 2026-10-27 the only committed change is an off-topic file the user asked for: `docs/notes/helios-future-resume.tex`, an illustrative future-state resume. Lines tagged `% TARGET` are aspirational placeholders, not claims; `% FACT` lines were true of the repo at the time. It is unrelated to project acceptance.

## Recent verified work

- `.mise.toml` pins the project toolchain; `mise ls` showed the pinned tools installed in this checkout.
- Gitleaks scanned four commits on 2026-10-14 and reported no leaks.
- `buf generate` succeeded. `buf lint` failed because `TelemetryIngest` in `apps/_proto/trackline/v1/telemetry.proto` does not meet the configured service-name suffix rule.
- `go test ./...` compiled `edge-gateway`, `ingest`, and `device-sim` after protobuf generation. The packages reported no test files.
- On 2026-10-27, `docker compose config --quiet` passed for the current Compose file (syntax only).
- 2026-11-17 (Windows, Docker Desktop 29.8.2, 7.398 GiB engine limit): `HEL-0.4.1` checks all passed. The assistant observed a healthy cold start of `redis`, `postgres_db` and `kafka` and no ZooKeeper service; the operator reported CLI connectivity, persistence across `docker compose restart`, and data cleared by `docker compose down -v`. Idle memory was about 336 MiB for the three services. Evidence: `ops/evidence/HEL-0.4.1-data-layer.md`.
- 2026-11-25 (macOS, OrbStack running): `HEL-0.4.3` is in progress, with only the Kafka step started and nothing verified. Observed: `apps/fleet-api/.dev-keys` has `private.pem` and `jwks.json`; `apps/pipeline/Dockerfile`, `apps/.dockerignore` and `.env` do not exist; the uncommitted `docker-compose.yml` adds `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"` to `kafka` and a `kafka-topics-init` stub that is still non-functional (only `image: apache/kafka` and `depends_on`, so it runs the image's default entrypoint, which starts a second broker). `docker ps -a` showed `kafka` (healthy), `helios-kafka-topics-init-1` (Up, i.e. the stub running as a broker) and an exited `kafka-data-init`. A `kafka-topics.sh --list` was run and exited 0, but its output was not captured, so whether the 10 topics exist is unknown. The assistant gave the operator a step-by-step guide for steps 1 to 7 and Stage 0 step 2; the operator is applying it by hand.
- 2026-11-25 (Windows): `HEL-0.4.3` Stage 0 step 1 done. `bash scripts/dev-token-keys.sh` (run with Git Bash at `C:\Program Files\Git\bin\bash.exe`; the `bash` on the PATH is the WSL launcher) wrote `apps/fleet-api/.dev-keys/private.pem` and `jwks.json` (`kid=dev-1`, `RS256`). `git check-ignore -v` shows both are ignored (`.gitignore:21`). `fleet-api` reads them from `/src/.dev-keys`, which matches the Compose mount. The keys are local only; regenerate them on another machine with the same script.
- 2026-11-25: Docker Desktop engine 29.8.2 was running, no Helios containers were running, and the `helios_pgdata`, `helios_kafkadata` and `helios_rcdata` volumes still existed. `apps/pipeline/Dockerfile` still does not exist. The `docker compose up -d` the operator ran on 2026-11-24 from the `scripts` folder exited 1; its cause was not investigated (compose commands must run from the repository root).
- 2026-11-20 (macOS, OrbStack not running): the checkout was fast-forwarded to the remote's `HEL-0.4.1`/`HEL-0.4.2` handoff commit with no local changes; `git remote -v` lists only `origin` (GitHub). `docker-compose.yml` is unchanged (5721 bytes, 12 services, no `pipeline`), and `docker-compose.obs.yml` and `Taskfile.yml` are empty. Nothing was re-verified on this machine.
- 2026-11-17: `HEL-0.4.2` verified. Flyway applied versions 1 (init), 2 (seed tenants) and 3 (notification delivery idempotency), all `success = t`. Seed counts: 3 tenants (one each free, pro, enterprise), 7 fleets, 150 vehicles, 150 devices, 5 geofences. Idempotent rerun passed; a temporary `V99__bad.sql` failed with SQL state 42601, was rolled back, and Flyway exited 1 with no version 99 row left (the file was then deleted); a `down -v` rebuild recreated the volume and reproduced the same counts (checked by migration timestamps). Evidence: `ops/evidence/HEL-0.4.2-schema-seed.md`. The data layer and Flyway were still up and healthy at the end of the session.

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
- The current `docker-compose.yml` contains Redis, PostgreSQL, Kafka, Flyway, the six Phase 0 service images, Mailpit, and a webhook sink. The data layer and Flyway were run on 2026-11-17 (see above). The application services (`fleet-api`, `edge-gateway`, `web`, `ingest`, `notifier`, `device-sim`) were not started under Compose; their health, data flow, and load acceptance remain unverified.
- On 2026-11-17 on this Windows machine, `apps/pipeline/Dockerfile` did not exist and only `kshitizmichael/ingest:dev` and `kshitizmichael/web:amd64` were present locally (the dev keys have since been generated, see above). Compose references `:dev` tags for all six services; images pushed before 2026-10-27 may not contain the new application code, so confirm or rebuild them.
- `scripts/verify-image-standard-amd64.sh` is a new operator-written copy of the arm64 verifier with `architecture=('amd64')`; it has the same gaps and has not been run or reviewed beyond reading it.
- `Taskfile.yml` and `docker-compose.obs.yml` were empty on 2026-11-17. `scripts/verify-image-standard.sh` is a partial script: arm64 only, checks the OOM flag, a size message and a non-root user, but has no no-shell check, no multi-arch check, and an oversize image only prints a message instead of failing. It does not satisfy `HEL-0.3.4`. Do not claim these acceptance checks pass based only on image files or the user's report.
- Docker Desktop is the engine on this Windows machine and was running on 2026-11-17; the macOS machine uses OrbStack.

## Next task

Operator work, following the Phase 0 order in `docs/03_ROADMAP.md` and `backlog/phase-0.yaml`. `HEL-0.4.1` and `HEL-0.4.2` have all checks recorded. The single next step is `HEL-0.4.3` (wire the application services into Compose), done one layer at a time, verifying each before the next. The full staged plan, with commands and checks for every stage, is in `docs/HEL-0.4.3-compose-wiring.md`. The assistant guides; the operator writes the Compose and Dockerfile changes unless they ask otherwise.

**Current position:** the work is at step 1 (Kafka topics). Replace the `kafka-topics-init` stub with a version that sets `restart: "no"`, `entrypoint: ["/bin/sh","-c"]` and a `command` script running `kafka-topics.sh --create --if-not-exists` for each topic. Then recreate it and confirm it exits 0 and lists the 10 topics plus `__consumer_offsets`, and that a rerun is idempotent. Stage 0 step 2 below is also still open.

**Stage 0 (prerequisites):** step 1 (dev keys) is done on both machines (keys are local only). **Next is step 2:** run `docker compose pull`, check each `:dev` image's creation date against 2026-10-27, and for any missing or stale image add a `build:` block (Go services `ingest`, `device-sim`, `pipeline` use `context: ./apps` with `dockerfile: <service>/Dockerfile`; `edge-gateway`, `fleet-api`, `notifier`, `web` use their own folder). Also add `apps/.dockerignore` excluding `**/node_modules`, `**/target`, `**/.next`, `**/dist` and `**/.dev-keys`, because the `./apps` context would otherwise send the private key to the build daemon.

Then, in order, each verified before the next (detail in `docs/HEL-0.4.3-compose-wiring.md`):

1. **Kafka topics:** set `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"` on `kafka` (it currently defaults to on) and add a one-shot `kafka-topics-init` creating the 10 topics from `apps/_contracts/kafka-topics.md` §1 (1 partition, replication factor 1; retention 24 h for `telemetry.raw`, 7 d for the three `events.*`, `notifications.requested` and `notifications.delivered`, 30 d for `alerts.raised` and both DLQs, 90 d for `usage.recorded`).
2. `REDIS_ADDR: redis:6379` on `fleet-api` and `ingest`.
3. A `pipeline` service: new `apps/pipeline/Dockerfile` (modelled on `ingest`'s, `apps/` context, `./cmd/pipeline`), env `KAFKA_BROKERS`, `DATABASE_URL`, `REDIS_ADDR`, `ROLES` unset. Open decision: the Go images have no shell, so choose how to health-check them (a `-healthcheck` flag in the binary, `service_started`, or a sidecar).
4. Apply `apps/_db/load/load-fleet.sql` outside the migration folders and confirm 500 vehicles.
5. `notifier`: `WEBHOOK_SECRET` (untracked `.env`) and `SMS_GATEWAY_URL: http://webhook-sink:8080`.
6. Credentials and exposure: move Postgres credentials into an untracked `.env` with a committed `.env.example`; keep `fleet-api` unpublished and `/internal/*` unproxied; bind Postgres and Redis to `127.0.0.1`.
7. Acceptance: cold `docker compose up` with health-based ordering and no manual steps, total memory under 8 GB, dashboard renders data. Record evidence in `ops/evidence/HEL-0.4.3-*.md`.

`Taskfile.yml` remains empty, so the one-command workflow and clean-rebuild criteria are still open.

Wiring reference (each service README lists its variables):

- A `pipeline` service (new Dockerfile, build context `apps/`) with `KAFKA_BROKERS`, `DATABASE_URL`, `REDIS_ADDR`; and `REDIS_ADDR` on `fleet-api` and `ingest`.
- Migration `V3` is already applied by Flyway. Apply `apps/_db/load/load-fleet.sql` for the 500-device fleet (it is not mounted in Compose). All contract Kafka topics must exist (auto-create is off by design).
- `WEBHOOK_SECRET` and an SMS sink (`SMS_GATEWAY_URL`) on `notifier`; `CORS_ALLOWED_ORIGINS` on the gateway; the gateway must reach `fleet-api` (API-key verification); `/internal/*` on `fleet-api` must not be exposed.
- Acceptance evidence to collect: device-sim at 500 devices shows moving markers on `/map`, `/vehicles/{id}` shows history, a scenario produces an alert and a Mailpit message, `usage_daily` fills.

Phase 2 deployables (`stream-proc`, `billing-batch`, `eta-ml`) need Dockerfiles and runtime design (checkpoint storage, CronJob policy, model artifact storage); they are not Phase 0 blockers. The container-image verifier discrepancy above remains visible; do not quietly mark its acceptance criteria as verified.

## Other open discrepancies

- The committed `.githooks/pre-commit` exists, but `core.hooksPath` was unset in the last check; the hook was therefore not confirmed active.
- `README.md`, the roadmap baseline/next-action section, and `docs/SCAFFOLD_STATUS.md` contain stale status claims. Refresh local progress documentation when acceptance evidence is available. Do not update ADO unless explicitly requested.
- No Phase 0 end-to-end stack validation under the user's Compose stack has been done by the assistant; the 2026-10-27 application tests used a throwaway environment, not Compose.
- Compose follow-ups: `apache/kafka:latest` and `flyway/flyway:latest` are unpinned; Postgres and Redis publish on all interfaces (`0.0.0.0`) while Kafka binds to `127.0.0.1`; Postgres credentials are hardcoded, which `HEL-0.4.3` forbids.
- ADO: the user asked on 2026-10-27 to update ADO (org `Kugleblitz712`, project `Ferrai`). It was **not done**: the Azure CLI sign-in had expired (`AADSTS70043`), and re-authenticating needs an interactive `az login --tenant 36da45f1-dd2c-4d1f-af13-5abe46b99921 --scope 499b84ac-1321-427f-aa17-267ca6975798/.default`. No ADO read or write happened. `scripts/backlog-sync.sh` is empty and the backlog YAML has no ADO ids, so work items must be found by title (`HEL-0.4.x`). Per `docs/06_WAYS_OF_WORKING.md` ADO owns state; add comments or create items only on request, and change states only for stories the user names.
- On the macOS machine on 2026-11-06 Docker was not running (OrbStack socket missing); `docker-compose.yml` was unchanged (12 services, no `pipeline`), `docker-compose.obs.yml`, `Taskfile.yml` and the scripts `nuke.sh`, `seed-load.sh`, `restore-drill.sh`, `backlog-sync.sh` were empty.
