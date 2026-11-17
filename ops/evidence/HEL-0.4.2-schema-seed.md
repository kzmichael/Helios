# HEL-0.4.2 Evidence: schema migration and seed data

Date: 2026-11-17
Environment: Windows, Docker Desktop engine 29.8.2 (linux/amd64).
Runner: Flyway (`flyway/flyway:latest`) via `docker compose up flyway`, locations `apps/_db/migrations` and `apps/_db/seed`.

## Results

| Check | Result | Source |
| --- | --- | --- |
| Migrations run on a clean database | Pass: `flyway` container exited 0 | Observed by assistant (`docker compose ps -a`) |
| Migration state is visible and queryable | Pass: `flyway_schema_history` lists versions 1, 2, 3, all `success = t` | Operator output, re-observed by assistant |
| Seed covers Free, Pro and Enterprise tiers | Pass: one tenant per tier | Operator output, re-observed by assistant |
| Idempotent on re-run | Pass: Flyway validated 3 migrations, reported schema version 3, "No migration necessary", exit 0 | Operator output |
| A failing migration halts startup | Pass: temporary `V99__bad.sql` (invalid SQL) failed with SQL state 42601, Flyway reported "Changes successfully rolled back" and exited with code 1. Afterwards `flyway_schema_history` still held only versions 1-3, seed counts were unchanged, and the bad file was removed | Operator output; history, counts and file removal checked by assistant |
| Rebuild from an empty volume reproduces the same state | Pass: after `down -v` the `helios_pgdata` volume was recreated (11:53:45Z) and versions 1-3 were applied at 11:54:00-11:54:01 UTC; Flyway exited 0; counts again 3 tenants, 7 fleets, 150 vehicles, 150 devices, 5 geofences | Operator output; timestamps and container state checked by assistant |
| Recovery after removing the bad migration | Pass: Flyway validated 3 migrations, reported schema version 3, "No migration necessary", exit 0 | Operator output |

## Migration history

| Rank | Version | Description | Success |
| ---: | --- | --- | --- |
| 1 | 1 | init | true |
| 2 | 2 | seed tenants | true |
| 3 | 3 | notification delivery idempotency | true |

## Seed counts (first run)

| Tenants | Fleets | Vehicles | Devices | Geofences |
| ---: | ---: | ---: | ---: | ---: |
| 3 | 7 | 150 | 150 | 5 |

Tiers: enterprise 1, free 1, pro 1.

## Open points

- All `HEL-0.4.2` acceptance checks now have evidence.
- The optional 500-device load file `apps/_db/load/load-fleet.sql` is not mounted in Compose and was not applied.
- `flyway/flyway:latest` is unpinned.
