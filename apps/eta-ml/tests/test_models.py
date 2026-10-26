from __future__ import annotations

import math
from datetime import datetime, timedelta, timezone

import numpy as np
import pytest

from eta_ml import train
from eta_ml.features import Point
from eta_ml.geo import haversine_m, heading_change_deg
from eta_ml.model import BaselineModel, ModelStore


def synthetic_windows(n: int, seed: int = 1):
    """Trucks are slower at rush hour, vans faster, everything capped by the limit."""
    rng = np.random.default_rng(seed)
    rows = []
    start = datetime(2026, 9, 1, tzinfo=timezone.utc)
    for i in range(n):
        ts = start + timedelta(minutes=5 * int(rng.integers(0, 24 * 12 * 14)))
        vtype = ["truck", "van", "car"][i % 3]
        limit = {"truck": 60.0, "van": 80.0, "car": 100.0}[vtype]
        rush = 1 if ts.hour in (8, 9, 17, 18) else 0
        speed = limit * (0.7 - 0.3 * rush) + rng.normal(0, 2)
        rows.append((ts, vtype, limit, max(speed, 5.0)))
    return rows


def synthetic_steps(n_vehicles: int = 20, per_vehicle: int = 80, seed: int = 2):
    rng = np.random.default_rng(seed)
    rows = []
    for v in range(n_vehicles):
        lat, lon, heading, t = 19.0, 72.8, rng.uniform(0, 360), 1_790_000_000.0
        for _ in range(per_vehicle):
            speed = max(0.0, rng.normal(45, 8))
            t += 5
            heading = (heading + rng.normal(0, 3)) % 360
            d = speed / 3.6 * 5
            lat += d * math.cos(math.radians(heading)) / 111_000
            lon += d * math.sin(math.radians(heading)) / 111_000
            rows.append((t, lat, lon, speed, heading, f"v{v}"))
    return rows


@pytest.fixture(scope="module")
def trained():
    return train.fit(synthetic_windows(1500), synthetic_steps(), min_rows=500)


def test_haversine_known_distance():
    # Mumbai to Pune is about 119 km as the crow flies.
    d = haversine_m(19.0760, 72.8777, 18.5204, 73.8567) / 1000
    assert 115 < d < 125


def test_heading_change_wraps():
    assert heading_change_deg(350, 10) == 20
    assert heading_change_deg(0, 180) == 180
    assert heading_change_deg(90, 90) == 0


def test_eta_model_learns_the_rush_hour_slowdown(trained):
    off_peak = trained.predict_speed_kph(datetime(2026, 9, 9, 12, tzinfo=timezone.utc), "truck", 60)
    rush = trained.predict_speed_kph(datetime(2026, 9, 9, 8, 30, tzinfo=timezone.utc), "truck", 60)
    assert off_peak > rush + 8
    van = trained.predict_speed_kph(datetime(2026, 9, 9, 12, tzinfo=timezone.utc), "van", 80)
    assert van > off_peak


def test_predicted_speed_never_exceeds_the_limit_or_drops_below_the_floor(trained):
    for hour in range(24):
        s = trained.predict_speed_kph(datetime(2026, 9, 9, hour, tzinfo=timezone.utc), "truck", 40)
        assert 5 <= s <= 40


def test_training_reports_metrics_better_than_a_mean_predictor(trained):
    m = trained.info.metrics
    assert m["eta_speed_mae_kph"] < m["eta_speed_mae_mean_predictor_kph"]


def test_not_enough_data_is_refused_not_trained():
    with pytest.raises(train.NotEnoughData):
        train.fit(synthetic_windows(50), synthetic_steps(2, 20), min_rows=500)


def track(jump: bool):
    pts = []
    lat, lon, t = 19.0, 72.8, 1_790_000_000.0
    for i in range(30):
        t += 5
        lat += 0.0005  # 55 m in 5 s = 40 km/h, consistent with the reported speed
        if jump and i == 15:
            lat += 0.5  # ~55 km in five seconds
        pts.append(Point(t, lat, lon, 40.0, 0.0))
    return pts


def test_trained_model_flags_a_position_jump(trained):
    clean = trained.anomaly_scores(track(jump=False))
    jumpy = trained.anomaly_scores(track(jump=True))
    assert max(jumpy) > 0.99
    assert max(jumpy) > max(clean)
    assert int(np.argmax(jumpy)) in (14, 15)


def test_baseline_flags_a_position_jump_and_not_a_clean_track():
    base = BaselineModel()
    assert max(base.anomaly_scores(track(jump=True))) == 1.0
    assert max(base.anomaly_scores(track(jump=False))) < 0.2


def test_artifact_round_trips_and_the_pointer_is_atomic(trained, tmp_path):
    store = ModelStore(tmp_path)
    assert store.load_current() is None

    store.publish(trained)
    loaded = store.load_current()
    assert loaded.info.version == trained.info.version
    when = datetime(2026, 9, 9, 8, 30, tzinfo=timezone.utc)
    assert loaded.predict_speed_kph(when, "truck", 60) == trained.predict_speed_kph(when, "truck", 60)
    assert not list(tmp_path.glob(".current-*"))  # no temp file left behind


def test_anomaly_dataset_never_builds_a_step_across_two_vehicles():
    rows = synthetic_steps(n_vehicles=3, per_vehicle=10)
    steps = train.build_anomaly_dataset(rows)
    assert len(steps) == 3 * 9  # 10 points per vehicle -> 9 steps, never 29
