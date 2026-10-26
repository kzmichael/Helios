"""Feature construction, shared by training and serving so they cannot drift apart."""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Sequence

import numpy as np

from .geo import haversine_m, heading_change_deg

VEHICLE_TYPES = ("truck", "van", "car", "bike", "trailer")

ETA_FEATURES = [
    "hour_sin",
    "hour_cos",
    "weekend",
    *[f"type_{t}" for t in VEHICLE_TYPES],
    "speed_limit_kph",
]

ANOMALY_FEATURES = ["speed_kph", "accel_ms2", "turn_rate_dps", "implied_vs_reported", "gap_s"]


def eta_features(depart: datetime, vehicle_type: str, speed_limit_kph: float) -> list[float]:
    """One row of ETA model input. Time is read in UTC so training and serving agree."""
    depart = depart.astimezone(timezone.utc)
    angle = 2 * math.pi * (depart.hour + depart.minute / 60) / 24
    one_hot = [1.0 if vehicle_type == t else 0.0 for t in VEHICLE_TYPES]
    return [math.sin(angle), math.cos(angle), 1.0 if depart.weekday() >= 5 else 0.0, *one_hot, float(speed_limit_kph)]


@dataclass(frozen=True)
class Point:
    t: float  # unix seconds
    lat: float
    lon: float
    speed_kph: float
    heading_deg: float = 0.0


def anomaly_rows(points: Sequence[Point]) -> np.ndarray:
    """
    One feature row per point after the first, describing the step from the previous point.

    implied_vs_reported compares the speed the GPS positions imply with the speed the
    device reported: a jump (a position teleporting) makes it large while the reported
    speed stays ordinary, which is exactly what a bad fix looks like.
    """
    rows: list[list[float]] = []
    for prev, cur in zip(points, points[1:]):
        dt = max(cur.t - prev.t, 1e-3)
        implied_kph = haversine_m(prev.lat, prev.lon, cur.lat, cur.lon) / dt * 3.6
        accel = (cur.speed_kph - prev.speed_kph) / 3.6 / dt
        turn = heading_change_deg(prev.heading_deg, cur.heading_deg) / dt
        ratio = implied_kph / max(cur.speed_kph, 5.0)
        rows.append([cur.speed_kph, accel, turn, min(ratio, 50.0), dt])
    return np.asarray(rows, dtype=float).reshape(-1, len(ANOMALY_FEATURES))
