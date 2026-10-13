# ingest

The telemetry sink. Devices stream GPS points here over gRPC; every valid point
is published to Kafka's `trackline.telemetry.raw` topic and the service holds no
state of its own. See `apps/_contracts/kafka-topics.md`.

## Why it holds no state

Statelessness is what lets you run ten copies behind a load balancer with no
coordination. All the interesting state — last known position, history,
aggregation — lives in consumers downstream (`position-updater`,
`telemetry-archiver`, `stream-proc`), not here. If `ingest` needed shared state,
scaling it would require a distributed lock, and that's a much harder problem than
the one it's solving.

## Delivery guarantee

`kafka.RequireAll` + a single in-flight write per call means a lost telemetry
point becomes a returned error, not a silent gap. `PublishBatch`'s `batch_id`
deduplication is **in-memory in Phase 0** — restarting this service resets it.
Making that durable (Redis-backed) is a Phase 2 concern once you have a shared
cache tier to lean on.

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `LISTEN_ADDR` | `:9090` | gRPC listen address |
| `KAFKA_BROKERS` | `kafka:9092` | Comma-separated broker list |

## Building it

Requires generated code from the proto contracts:

```bash
cd apps/_proto && buf generate      # produces apps/gen/go/trackline/v1/*
cd ../ingest && go mod tidy
go run ./cmd/ingest
```

**Not yet run** — no Go toolchain confirmed and no Kafka broker up yet
(`HEL-0.1.2`, `HEL-0.4.1`). `internal/validate` has no dedicated test file yet;
writing one is a good first exercise once the toolchain exists.

## What's deliberately not implemented yet

- MQTT transport for constrained devices — Phase 2
- Backpressure/load shedding (`limits.md` §6) — Phase 6, needs load tests to tune against
- Durable batch deduplication — Phase 2, once Redis is wired here too
