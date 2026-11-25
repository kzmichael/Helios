# HEL-0.4.3: Wire the application services into Compose

Status as of 2026-11-24. Backlog story: `HEL-0.4.3` in `backlog/phase-0.yaml`. Follow-on: `HEL-0.4.4` (500-device load) and `HEL-0.4.5` (local notification sinks).

Goal: `docker compose up` reaches a fully healthy state from cold, with the six Phase 0 services running against the data layer, and the web dashboard rendering data from `fleet-api`.

Work one layer at a time. Each stage ends with a check; do not start the next stage until it passes. Run every `docker compose` command from the repository root.

## Already done

- `HEL-0.4.1` (data layer) and `HEL-0.4.2` (schema and seed) have evidence in `ops/evidence/`.
- Stage 0, step 1: the dev key pair exists at `apps/fleet-api/.dev-keys/` (`private.pem`, `jwks.json` with `kid=dev-1`, `RS256`). The folder is ignored by git (`.gitignore:21`). Generate it again on another machine with `bash scripts/dev-token-keys.sh`; on Windows use Git Bash (`C:\Program Files\Git\bin\bash.exe`), not the WSL launcher on the PATH.

## Known gaps before starting

- `apps/pipeline/Dockerfile` does not exist, and Compose has no `pipeline` service.
- Compose uses `:dev` image tags for all six services. Images pushed before 2026-10-27 may not contain the current application code.
- `kafka` has no `KAFKA_AUTO_CREATE_TOPICS_ENABLE` setting, so auto-create is on; the contract requires it off.
- `fleet-api` and `ingest` have no `REDIS_ADDR`.
- `notifier` has no `WEBHOOK_SECRET` or `SMS_GATEWAY_URL`.
- Postgres credentials (`myuser` / `mypassword`) are hardcoded; Postgres and Redis publish on `0.0.0.0`.
- `docker-compose.yml` has no `.env` handling and no `build:` blocks.

## Stage 0: prerequisites

1. **Dev keys:** done (see above).
2. **Service images.** Start Docker Desktop, then:

   ```powershell
   docker compose pull
   docker inspect kshitizmichael/fleet-api:dev --format '{{.Created}}'
   ```

   Repeat the inspect for each of the six services. Anything that fails to pull, or was created before 2026-10-27, must be rebuilt.
3. **Add `build:` blocks** so Compose builds from current source:

   | Service | `context` | `dockerfile` |
   | --- | --- | --- |
   | `ingest` | `./apps` | `ingest/Dockerfile` |
   | `device-sim` | `./apps` | `device-sim/Dockerfile` |
   | `edge-gateway` | `./apps/edge-gateway` | `Dockerfile` |
   | `fleet-api` | `./apps/fleet-api` | `Dockerfile` |
   | `notifier` | `./apps/notifier` | `Dockerfile` |
   | `web` | `./apps/web` | `Dockerfile` |

   The Go services built from `./apps` need the generated protobuf code in `apps/gen/go`. It is git-ignored; run `buf generate` in `apps/_proto` if it is missing.
4. **Create `apps/.dockerignore`** excluding `**/node_modules`, `**/target`, `**/.next`, `**/dist` and `**/.dev-keys`. Without it the `./apps` context sends those folders, including the private key, to the build daemon. `fleet-api` and `web` have their own `.dockerignore`, but it applies only when their own folder is the context.

Check: `docker compose build` completes without warnings.

## Stage 1: Kafka topics

1. Set `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"` on `kafka`.
2. Add a one-shot `kafka-topics-init` service. It waits for `kafka` to be healthy, then creates each topic with `kafka-topics.sh --create --if-not-exists`, 1 partition, replication factor 1, and `--config retention.ms=<value>`.
3. Topics and retention (from `apps/_contracts/kafka-topics.md` section 1):

   | Topic | Retention | `retention.ms` |
   | --- | --- | --- |
   | `trackline.telemetry.raw` | 24 h | `86400000` |
   | `trackline.events.geofence` | 7 d | `604800000` |
   | `trackline.events.violation` | 7 d | `604800000` |
   | `trackline.events.vehicle` | 7 d | `604800000` |
   | `trackline.alerts.raised` | 30 d | `2592000000` |
   | `trackline.notifications.requested` | 7 d | `604800000` |
   | `trackline.notifications.delivered` | 7 d | `604800000` |
   | `trackline.usage.recorded` | 90 d | `7776000000` |
   | `trackline.telemetry.dlq` | 30 d | `2592000000` |
   | `trackline.notifications.dlq` | 30 d | `2592000000` |

Check: `kafka-topics.sh --list` shows all 10 topics, `--describe` shows the retention, and running the init service again succeeds.

## Stage 2: Redis for `fleet-api` and `ingest`

Add `REDIS_ADDR: redis:6379` to both services, with `depends_on: redis: condition: service_healthy`. The gateway already has it.

Check: both services start and stay up (`docker compose ps`).

## Stage 3: the `pipeline` service

1. Write `apps/pipeline/Dockerfile`, modelled on `apps/ingest/Dockerfile`: `apps/` build context, build `./cmd/pipeline`, Distroless static nonroot runtime.
2. Add the service with `KAFKA_BROKERS: kafka:9092`, `DATABASE_URL`, `REDIS_ADDR: redis:6379`. Leave `ROLES` unset so every role runs in one process. It exposes `/healthz` and `/readyz` on `:8082`.
3. Health check decision. The Go images have no shell, so a `curl` check is not possible. Recommended: add a small `-healthcheck` flag to the pipeline binary that requests its own `/readyz` and exits 0 or 1, and use it as the Compose health check. For `ingest` and `device-sim`, use `condition: service_started` and record that as an accepted exception in the evidence file.
4. Depend on `kafka-topics-init` (completed), `flyway` (completed) and `redis` (healthy).

Check: the service is up, `/readyz` returns OK, and its logs show consumer groups joined.

## Stage 4: the 500-device fleet

1. Apply the opt-in load file, kept outside the migration folders:

   ```powershell
   Get-Content apps\_db\load\load-fleet.sql | docker exec -i postgres_db psql -U myuser -d mydatabase
   docker exec postgres_db psql -U myuser -d mydatabase -c "SELECT count(*) FROM vehicle;"
   ```

   Expect 500.
2. Start order: `pipeline` (its `registry-sync` role fills Redis), then `ingest` with `DEVICE_AUTH: required`, then `device-sim` with `DEVICE_COUNT: "500"`.

Check: telemetry rows grow (`SELECT count(*) FROM telemetry;` twice, a minute apart) and `vehicle_position_latest` updates.

## Stage 5: `notifier`

1. Set `WEBHOOK_SECRET` from an untracked `.env`. Without it webhook deliveries are dead-lettered.
2. Set `SMS_GATEWAY_URL: http://webhook-sink:8080`. The existing echo sink works.
3. Email already goes to Mailpit.

Check: run `device-sim` with `SCENARIO=speeding` and confirm an alert, a Mailpit message (`http://localhost:8025`), and a captured webhook (`docker logs webhook-sink`).

## Stage 6: credentials and exposure

1. Move the Postgres credentials into an untracked `.env` (`.env*` is already ignored) and commit a `.env.example` with placeholder values. Reference them as `${VAR}` in Compose.
2. Bind Postgres and Redis to `127.0.0.1`.
3. Keep `fleet-api` without a published port. The gateway proxies only `/v1/*`, so `/internal/*` stays unreachable from outside.
4. Dev tokens come only from `fleet-api` (`GET /dev/token/:tenantId/:subjectId/:tier`, not proxied by the gateway). Mint test tokens from inside the network, or let the `web` service fetch them.

Check: `docker compose config` shows no plaintext credentials, and `docker port` shows `127.0.0.1` for Postgres and Redis.

## Stage 7: acceptance

- `docker compose down -v`, then `docker compose up` from cold, with no manual steps and ordering enforced by health checks, not sleeps.
- `docker stats --no-stream`: total memory under 8 GB (the Docker Desktop engine here is capped at 7.398 GiB).
- Moving markers on `/map` and history on `/vehicles/{id}`.
- `SCENARIO=speeding` produces an alert and a Mailpit message.
- `usage_daily` fills.
- Record commands, results and the memory figure in `ops/evidence/HEL-0.4.3-compose-wiring.md`, in the same format as the earlier evidence files.

## Later, not part of this story

- `Taskfile.yml` is empty, so the one-command workflow (`HEL-0.6.2`) and clean-rebuild criteria remain open.
- Pin `apache/kafka`, `flyway/flyway` and other `:latest` tags.
- `stream-proc`, `billing-batch` and `eta-ml` have no Dockerfiles or Compose services; they are Phase 2 deployables.
