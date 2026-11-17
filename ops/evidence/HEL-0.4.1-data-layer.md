# HEL-0.4.1 Evidence: local data layer

Date: 2026-11-17
Environment: Windows, Docker Desktop engine 29.8.2 (linux/amd64), 7.398 GiB engine memory limit.
Compose file: `docker-compose.yml` (services `redis`, `postgresql`, `kafka`, plus one-shot `kafka-data-init`).

## Results

| Check | Result | Source |
| --- | --- | --- |
| Cold start reaches healthy with no manual steps | Pass: `redis`, `postgres_db`, `kafka` all `Up (healthy)`; `kafka-data-init` `Exited (0)` | Observed by assistant (`docker compose ps`) |
| Kafka in KRaft mode, no ZooKeeper | Pass: `docker compose ps --services` lists only `kafka`, `postgresql`, `redis` | Observed by assistant |
| CLI client connects to each service | Pass | Reported by operator |
| Data survives `docker compose restart` (Redis key, Postgres table row, Kafka topic) | Pass | Reported by operator |
| `docker compose down -v` clears the data | Pass: the Redis key, Postgres table, and Kafka topic were gone after `down -v` and a fresh start | Reported by operator |

## Memory and CPU (idle, `docker stats --no-stream`)

| Container | CPU % | Memory |
| --- | ---: | ---: |
| kafka | 2.46% | 300 MiB |
| postgres_db | 0.03% | 29.83 MiB |
| redis | 0.20% | 6.066 MiB |
| **Total** | | **about 336 MiB** |

## Open points

- All `HEL-0.4.1` acceptance checks have evidence. Items below are follow-ups, not blockers for this story.
- `apache/kafka:latest` is unpinned, which conflicts with the image standard's "no `:latest`" rule.
- Postgres and Redis are published on all interfaces (`0.0.0.0`); Kafka is bound to `127.0.0.1`. Bind the first two to `127.0.0.1`.
- Postgres credentials (`myuser`/`mypassword`) are hardcoded in the Compose file; `HEL-0.4.3` forbids this.
