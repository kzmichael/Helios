# eta-ml

ETA prediction and anomaly scoring (Python 3.12+, FastAPI, scikit-learn), with a
batch retraining job. It is an internal service: it sits behind the cluster network
boundary and does no authentication of its own (`auth.md` §3).

## API

| Endpoint                 | Purpose                                                            |
| ------------------------ | ------------------------------------------------------------------ |
| `GET /healthz`           | Liveness, and which model is serving                               |
| `GET /v1/model`          | Model name, version, training time, row counts and metrics         |
| `POST /v1/model/reload`  | Pick up a newly published model now (otherwise checked every 30 s) |
| `POST /v1/eta`           | ETA between two points                                             |
| `POST /v1/anomaly/score` | Per-step anomaly scores for a track (2–1,000 points)               |

```bash
curl -s localhost:8000/v1/eta -H 'content-type: application/json' -d '{
  "from": {"lat": 19.076, "lon": 72.8777}, "to": {"lat": 18.5204, "lon": 73.8567},
  "vehicleType": "truck", "speedLimitKph": 60, "departAt": "2026-10-27T03:00:00Z"
}'
```

Every response says which model produced it.

## The models

- **ETA.** Gradient-boosted trees predict the _effective_ speed (stops included) a
  vehicle type sustains at a given hour and day, capped between 5 km/h and the speed
  limit. ETA is road distance over that speed; road distance is the great-circle
  distance times `ROAD_DISTANCE_FACTOR` (default 1.3). It is a speed model, not a
  routing engine.
- **Anomaly.** An isolation forest over per-step features (speed, acceleration, turn
  rate, implied-vs-reported speed, time gap), scored as "how unusual compared with the
  training data" in 0–1, combined by `max` with a rule-based plausibility score so a
  physically impossible step (a position jump, 8 g braking) is never missed. A step is
  anomalous at 0.99 by default; the request can set its own threshold.
- **Baseline.** With no trained model published the service serves a transparent rule
  set (60% of the limit; the plausibility rules) and says `"baseline"` in every
  response. It never refuses to answer for lack of a model.

## Training (the batch job)

```bash
DATABASE_URL=postgres://… MODEL_DIR=/models python -m eta_ml.train --since-days 7
```

It reads recent `telemetry` joined to `vehicle`, fits both models, reports MAE on a
held-out slice next to a mean-predictor MAE, and publishes
`MODEL_DIR/model-<UTC timestamp>.joblib` plus a `current.json` pointer, swapped with
one atomic rename. Rolling back is writing the previous artifact's name into
`current.json`. Exit codes: `0` published, `3` not enough data (the live model is left
alone), `1` error.

Artifacts are joblib (pickle) files: only load them from a directory the operator
controls. `MODEL_DIR` is the artifact store — a volume today, an object-storage mount
or sync later; that choice is yours.

## Configuration

| Variable               | Default    | Meaning                                           |
| ---------------------- | ---------- | ------------------------------------------------- |
| `MODEL_DIR`            | `./models` | Where artifacts and `current.json` live           |
| `DATABASE_URL`         | —          | Training only; the API never touches the database |
| `ROAD_DISTANCE_FACTOR` | `1.3`      | Great-circle to road-distance correction          |

## Run and test

```bash
python -m venv .venv && . .venv/bin/activate
pip install -r requirements-dev.txt
pytest                                    # 16 tests
uvicorn eta_ml.api:create_app --factory --host 0.0.0.0 --port 8000
```

CPU is enough for both training and serving at this scale; nothing here needs a GPU.

## Verified

The tests train on synthetic data with a known rush-hour pattern and assert the model
learns it, never exceeds the limit, beats a mean predictor, refuses to train on too
little data, round-trips through the store, and flags an injected position jump. Run
against the throwaway database's simulated telemetry it trained and served (its MAE
there is poor, which is honest: the simulator has no real daily traffic pattern, and
the metrics in `/v1/model` say so).

## What's deliberately not implemented

- Real routing or traffic data; per-tenant models; GPU training.
- Automatic promotion gates (a new model is published if it trained; comparing it
  with the live one before promoting is a policy decision for the operator).
- Metrics endpoints (Phase 3).
