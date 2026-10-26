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

| Profile     | Models                              | Exercises                                              |
| ----------- | ----------------------------------- | ------------------------------------------------------ |
| `commuter`  | Stop-start city driving             | The common case                                        |
| `long-haul` | Sustained highway speed, rare stops | Speeding rule tuning, long-running streams             |
| `idle`      | Mostly parked, low emission rate    | The "device offline" alert                             |
| `erratic`   | Noisy, implausible values           | `ingest`'s `validate.Point`, alert false-positive rate |

## Who the devices are

By default the simulator plays the **seeded fleet**: with `DEVICE_COUNT=500` it
drives the 500 provisioned devices (the seed's 150 plus the 350 that
`apps/_db/load/load-fleet.sql` provisions — opt-in, deliberately
outside the migration and seed directories). Device ids and bearer tokens are
derived from the seed's conventions, so `ingest` with `DEVICE_AUTH=required` accepts
them, and each vehicle starts near its fleet's depot so geofences and the map mean
something. `FLEET_PLACEMENT=false` falls back to scattering around
`START_LAT`/`START_LON`; devices beyond the provisioned set are unregistered and are
only accepted by an `ingest` that is not enforcing auth.

## Environment variables

| Variable                  | Default                           | Meaning                                                                   |
| ------------------------- | --------------------------------- | ------------------------------------------------------------------------- |
| `INGEST_ADDR`             | `ingest:9090`                     | Where to stream telemetry                                                 |
| `DEVICE_COUNT`            | `500`                             | Number of simulated vehicles                                              |
| `START_LAT` / `START_LON` | Bengaluru                         | Origin for unplaced devices                                               |
| `PROFILE_MIX`             | `commuter,long-haul,idle,erratic` | Cycled across devices in order                                            |
| `EMISSION_HZ`             | per-profile                       | Override the emission rate                                                |
| `START_SPREAD_SECONDS`    | `10`                              | Spread device start times so they do not all connect at once              |
| `FLEET_PLACEMENT`         | `true`                            | Start each vehicle near its fleet's depot                                 |
| `HONOR_BACKPRESSURE`      | `true`                            | Obey `retry_after_ms` from `ingest` (`false` models a misbehaving device) |
| `SCENARIO`                | —                                 | Play one scripted drive instead of the fleet (below)                      |

### Scenarios

`SCENARIO=<name>` drives a single seeded device along a fixed path so a specific alert
fires on demand instead of waiting for a random walk to cross a line:

| Scenario     | Raises                                                            |
| ------------ | ----------------------------------------------------------------- |
| `port-entry` | Meridian vehicle enters the Restricted Port Area (geofence enter) |
| `hub-exit`   | Meridian vehicle leaves the Mumbai Hub (geofence exit)            |
| `speeding`   | Atlas vehicle runs well over its limit for 90 s                   |
| `offline`    | Atlas vehicle reports for 20 s then goes silent for 8 minutes     |

### Chaos

All off by default; every one is seeded (`CHAOS_SEED`) so a failure can be reproduced.

| Variable                        | Effect                                                                                 |
| ------------------------------- | -------------------------------------------------------------------------------------- |
| `CHAOS_CLOCK_SKEW_SECONDS`      | Offsets the device clock of a fraction (`CHAOS_SKEW_FRACTION`, default 0.1) of devices |
| `CHAOS_PACKET_LOSS_PCT`         | Drops that percentage of points before they are sent                                   |
| `CHAOS_DUPLICATE_PCT`           | Sends that percentage of points twice                                                  |
| `CHAOS_DISCONNECT_MEAN_SECONDS` | Drops each device's stream at random, on average this often, and reconnects            |

## Running it

```bash
cd apps/_proto && buf generate
cd ../device-sim && go test ./... && DEVICE_COUNT=500 go run ./cmd/simulator
```

Movement is dead-reckoning, not a routing engine; it's realistic enough to exercise
geofences and speed rules, not a claim about real road networks. A 500-device run
against `ingest` and the pipeline produced roughly 500 points a second, and the unit
tests cover identity derivation, the chaos injectors and every scenario resolving to
a provisioned device.

## What's deliberately not implemented yet

- MQTT transport — only gRPC streaming. (`ingest` can accept MQTT, but the simulator
  does not publish it.)
- A CLI flag layer — everything is env-var configured for now; fine for compose,
  worth revisiting for k6-style parameterised load profiles in Phase 6
- Scaling past one process: ten thousand to a million devices is a Phase 6
  distributed-load exercise, not something one `go run` is meant to do
