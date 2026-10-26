"""Batch retraining: read recent telemetry, fit both models, publish a new artifact.

    python -m eta_ml.train --since-days 7

Reads DATABASE_URL (postgres://...) and writes into MODEL_DIR. Exit codes: 0 published,
3 not enough data to train (the live model is left alone), 1 anything else.
"""

from __future__ import annotations

import argparse
import logging
import os
import sys
from datetime import datetime, timezone

import numpy as np
from sklearn.ensemble import HistGradientBoostingRegressor, IsolationForest
from sklearn.metrics import mean_absolute_error
from sklearn.model_selection import train_test_split

from .features import ANOMALY_FEATURES, ETA_FEATURES, Point, anomaly_rows, eta_features
from .model import ModelInfo, ModelStore, TrainedModel

LOG = logging.getLogger("eta_ml.train")

EXIT_OK, EXIT_ERROR, EXIT_NOT_ENOUGH_DATA = 0, 1, 3

# Mean speed over each vehicle's 5-minute windows: a window spans stops and moving, so
# its mean is the *effective* speed an ETA should be computed with.
WINDOW_SQL = """
SELECT to_timestamp(floor(extract(epoch FROM t.recorded_at) / 300) * 300) AS window_start,
       v.vehicle_type, v.speed_limit_kph::float8, avg(t.speed_kph)::float8 AS mean_speed
FROM telemetry t JOIN vehicle v ON v.id = t.vehicle_id
WHERE t.recorded_at > now() - make_interval(days => %(days)s)
GROUP BY t.vehicle_id, 1, v.vehicle_type, v.speed_limit_kph
HAVING count(*) >= 5
LIMIT %(limit)s
"""

STEP_SQL = """
SELECT extract(epoch FROM recorded_at)::float8, lat, lon, speed_kph::float8, heading_deg::float8, vehicle_id::text
FROM telemetry
WHERE recorded_at > now() - make_interval(days => %(days)s)
ORDER BY vehicle_id, recorded_at
LIMIT %(limit)s
"""


def build_eta_dataset(rows) -> tuple[np.ndarray, np.ndarray]:
    x = [eta_features(r[0], r[1], r[2]) for r in rows]
    y = [r[3] for r in rows]
    return np.asarray(x, dtype=float), np.asarray(y, dtype=float)


def build_anomaly_dataset(rows) -> np.ndarray:
    """Rows are ordered by vehicle then time; steps are built per vehicle, never across two."""
    out: list[np.ndarray] = []
    batch: list[Point] = []
    current = None
    for ts, lat, lon, speed, heading, vehicle in rows:
        if vehicle != current:
            if len(batch) > 1:
                out.append(anomaly_rows(batch))
            batch, current = [], vehicle
        batch.append(Point(ts, lat, lon, speed, heading))
    if len(batch) > 1:
        out.append(anomaly_rows(batch))
    return np.vstack(out) if out else np.empty((0, len(ANOMALY_FEATURES)))


def fit(eta_rows, step_rows, *, min_rows: int, seed: int = 7) -> TrainedModel:
    x, y = build_eta_dataset(eta_rows)
    steps = build_anomaly_dataset(step_rows)
    if len(x) < min_rows or len(steps) < min_rows:
        raise NotEnoughData(f"need at least {min_rows} rows, have {len(x)} ETA windows and {len(steps)} steps")

    x_tr, x_te, y_tr, y_te = train_test_split(x, y, test_size=0.2, random_state=seed)
    eta = HistGradientBoostingRegressor(max_iter=120, learning_rate=0.1, random_state=seed).fit(x_tr, y_tr)
    mae = float(mean_absolute_error(y_te, eta.predict(x_te)))
    baseline_mae = float(mean_absolute_error(y_te, np.full_like(y_te, y_tr.mean())))
    eta.fit(x, y)  # the held-out slice was only for the metric; the shipped model sees everything

    forest = IsolationForest(n_estimators=150, contamination="auto", random_state=seed).fit(steps)
    quantiles = np.sort(forest.score_samples(steps))

    info = ModelInfo(
        name="hgb-eta+iforest",
        version=datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S"),
        trained_at=datetime.now(timezone.utc).isoformat(),
        rows={"eta_windows": int(len(x)), "anomaly_steps": int(len(steps))},
        metrics={"eta_speed_mae_kph": mae, "eta_speed_mae_mean_predictor_kph": baseline_mae},
    )
    return TrainedModel(eta, forest, quantiles, info)


class NotEnoughData(Exception):
    pass


def load_rows(database_url: str, days: int, limit: int):
    import psycopg  # imported late so the API never needs a database driver

    with psycopg.connect(database_url) as conn, conn.cursor() as cur:
        cur.execute(WINDOW_SQL, {"days": days, "limit": limit})
        windows = [(r[0], r[1], r[2], r[3]) for r in cur.fetchall()]
        cur.execute(STEP_SQL, {"days": days, "limit": limit})
        steps = cur.fetchall()
    return windows, steps


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--since-days", type=int, default=7)
    p.add_argument("--max-rows", type=int, default=2_000_000)
    p.add_argument("--min-rows", type=int, default=500)
    p.add_argument("--model-dir", default=os.environ.get("MODEL_DIR", "./models"))
    args = p.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    database_url = os.environ.get("DATABASE_URL")
    if not database_url:
        LOG.error("DATABASE_URL must be set")
        return EXIT_ERROR

    try:
        windows, steps = load_rows(database_url, args.since_days, args.max_rows)
        LOG.info("loaded %d windows and %d telemetry rows", len(windows), len(steps))
        model = fit(windows, steps, min_rows=args.min_rows)
    except NotEnoughData as e:
        LOG.error("not training: %s", e)
        return EXIT_NOT_ENOUGH_DATA
    except Exception:
        LOG.exception("training failed")
        return EXIT_ERROR

    artifact = ModelStore(args.model_dir).publish(model)
    LOG.info("published %s: %s", artifact, model.info.metrics)
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
