"""The models, and where they are kept.

Two models share one artifact:

* an ETA model: gradient-boosted trees predicting the effective speed (stops included)
  a vehicle of a given type sustains at a given hour and day, from which ETA is distance
  over speed;
* an anomaly model: an isolation forest over per-step features, whose raw score is turned
  into a 0..1 "how unusual compared with training data" figure, combined (max) with the
  rule-based plausibility score so impossible steps are never missed.

When no trained artifact exists the service runs on a transparent baseline instead of
refusing to answer, and says so in every response.
"""

from __future__ import annotations

import json
import os
import tempfile
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Protocol, Sequence

import joblib
import numpy as np

from .features import Point, anomaly_rows, eta_features

MIN_SPEED_KPH = 5.0
DEFAULT_LIMIT_KPH = 60.0
ANOMALY_THRESHOLD = 0.99


@dataclass
class ModelInfo:
    name: str
    version: str
    trained_at: str | None = None
    rows: dict[str, int] = field(default_factory=dict)
    metrics: dict[str, float] = field(default_factory=dict)


class Model(Protocol):
    info: ModelInfo

    def predict_speed_kph(self, depart: datetime, vehicle_type: str, speed_limit_kph: float | None) -> float: ...

    def anomaly_scores(self, points: Sequence[Point]) -> list[float]: ...


def rule_scores(rows: np.ndarray) -> list[float]:
    """Physical plausibility: how far a step is from something a road vehicle can do, 0..1."""
    out: list[float] = []
    for row in rows:
        _speed, accel, turn, implied_ratio, _gap = row
        # Each term saturates at 1 when the behaviour is clearly not a road vehicle.
        teleport = min(max((implied_ratio - 3.0) / 7.0, 0.0), 1.0)
        harsh = min(max((abs(accel) - 4.0) / 6.0, 0.0), 1.0)
        spin = min(max((turn - 45.0) / 90.0, 0.0), 1.0)
        out.append(float(max(teleport, harsh, spin)))
    return out


class BaselineModel:
    """Rules of thumb. Used until the first model is trained, and as the test reference."""

    info = ModelInfo(name="baseline", version="rules-1")

    #: Share of the speed limit a vehicle averages over a real journey (junctions, loading, traffic).
    EFFICIENCY = 0.6

    def predict_speed_kph(self, depart: datetime, vehicle_type: str, speed_limit_kph: float | None) -> float:
        limit = speed_limit_kph or DEFAULT_LIMIT_KPH
        return max(MIN_SPEED_KPH, limit * self.EFFICIENCY)

    def anomaly_scores(self, points: Sequence[Point]) -> list[float]:
        return rule_scores(anomaly_rows(points))


class TrainedModel:
    def __init__(self, eta_model, anomaly_model, score_quantiles: np.ndarray, info: ModelInfo):
        self.eta_model = eta_model
        self.anomaly_model = anomaly_model
        self.score_quantiles = score_quantiles  # sorted raw isolation-forest scores of the training set
        self.info = info

    def predict_speed_kph(self, depart: datetime, vehicle_type: str, speed_limit_kph: float | None) -> float:
        limit = speed_limit_kph or DEFAULT_LIMIT_KPH
        x = np.asarray([eta_features(depart, vehicle_type, limit)])
        predicted = float(self.eta_model.predict(x)[0])
        # A vehicle does not sustain more than its limit, nor crawl below a floor.
        return min(max(predicted, MIN_SPEED_KPH), max(limit, MIN_SPEED_KPH))

    def anomaly_scores(self, points: Sequence[Point]) -> list[float]:
        rows = anomaly_rows(points)
        if len(rows) == 0:
            return []
        raw = self.anomaly_model.score_samples(rows)  # lower = more anomalous
        # Share of training data that was *less* unusual than this step.
        rank = np.searchsorted(self.score_quantiles, raw, side="right") / len(self.score_quantiles)
        unusual = 1.0 - np.minimum(rank, 1.0)
        # The forest knows what is unusual *for this fleet*, but a far outlier scores only a
        # little worse than ordinary noise, so physically impossible steps are caught by rule.
        return [float(max(u, r)) for u, r in zip(unusual, rule_scores(rows))]

    # --- persistence -------------------------------------------------------------------

    def dump(self, path: Path) -> None:
        joblib.dump(
            {
                "eta_model": self.eta_model,
                "anomaly_model": self.anomaly_model,
                "score_quantiles": self.score_quantiles,
                "info": asdict(self.info),
            },
            path,
        )

    @staticmethod
    def load(path: Path) -> "TrainedModel":
        # joblib is pickle: only ever load artifacts from a directory this service's
        # operator controls (MODEL_DIR), never from a user-supplied path.
        blob = joblib.load(path)
        return TrainedModel(blob["eta_model"], blob["anomaly_model"], blob["score_quantiles"], ModelInfo(**blob["info"]))


class ModelStore:
    """
    A directory of artifacts plus a pointer file, current.json, naming the live one.
    Promoting a model is a single atomic rename of that pointer, so a reader never sees
    a half-written file, and rolling back is writing the old name back.
    """

    POINTER = "current.json"

    def __init__(self, directory: str | os.PathLike[str]):
        self.dir = Path(directory)

    def publish(self, model: TrainedModel) -> Path:
        self.dir.mkdir(parents=True, exist_ok=True)
        stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        artifact = self.dir / f"model-{stamp}.joblib"
        model.dump(artifact)

        pointer = {"artifact": artifact.name, "published_at": stamp, "info": asdict(model.info)}
        fd, tmp = tempfile.mkstemp(dir=self.dir, prefix=".current-", suffix=".json")
        with os.fdopen(fd, "w") as f:
            json.dump(pointer, f, indent=2)
        os.replace(tmp, self.dir / self.POINTER)
        return artifact

    def pointer_mtime(self) -> float | None:
        try:
            return (self.dir / self.POINTER).stat().st_mtime
        except FileNotFoundError:
            return None

    def load_current(self) -> TrainedModel | None:
        try:
            pointer = json.loads((self.dir / self.POINTER).read_text())
            return TrainedModel.load(self.dir / Path(pointer["artifact"]).name)
        except FileNotFoundError:
            return None
