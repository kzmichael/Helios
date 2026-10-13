# device-sim

Synthetic fleet of vehicles. Your load generator, chaos driver, and cost driver —
see `docs/01_APPLICATION.md` §1.4. Scales from a handful of devices to the
million-device load tests in Phase 6.

## Why four behaviour profiles instead of one

Identical simulated devices produce identical, boring load. Real fleets are a mix
of commuters, long-haul trucks, parked vehicles, and the occasional
misbehaving device. Mixing profiles (`PROFILE_MIX`) is what makes seeded
weaknesses (`docs/01_APPLICATION.md` §6) discoverable — a uniform load never
finds a hot partition, because it never creates one.

| Profile | Models | Exercises |
|---|---|---|
| `commuter` | Stop-start city driving | The common case |
| `long-haul` | Sustained highway speed, rare stops | Speeding rule tuning, long-running streams |
| `idle` | Mostly parked, low emission rate | The "device offline" alert |
| `erratic` | Noisy, implausible values | `ingest`'s `validate.Point`, alert false-positive rate |

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `INGEST_ADDR` | `ingest:9090` | Where to stream telemetry |
| `DEVICE_COUNT` | `500` | Number of simulated vehicles |
| `START_LAT` / `START_LON` | Bengaluru | Origin around which devices scatter |
| `PROFILE_MIX` | `commuter,long-haul,idle,erratic` | Cycled across devices in order |

## Running it

```bash
cd apps/_proto && buf generate
cd ../device-sim && go mod tidy
DEVICE_COUNT=500 go run ./cmd/simulator
```

**Not yet run** — depends on `ingest` being up. Movement is dead-reckoning, not a
routing engine; it's realistic enough to exercise geofences and speed rules, not
a claim about real road networks.

## What's deliberately not implemented yet

- Chaos injection (clock skew, packet loss, forced disconnects) — Phase 6,
  `internal/chaos/` per `docs/07_REPOSITORY_BLUEPRINT.md` §3.7
- MQTT transport — only gRPC streaming exists so far
- A CLI flag layer — everything is env-var configured for now; fine for compose,
  worth revisiting for k6-style parameterised load profiles in Phase 6
