"""HTTP API.

    GET  /healthz            liveness + which model is serving
    GET  /v1/model           model metadata
    POST /v1/model/reload    pick up a newly published artifact now
    POST /v1/eta             ETA between two points
    POST /v1/anomaly/score   per-step anomaly scores for a track

Internal service: it sits behind the cluster network boundary like fleet-api, and does
no authentication of its own (auth.md §3).

Run:  uvicorn eta_ml.api:create_app --factory --host 0.0.0.0 --port 8000
"""

from __future__ import annotations

import logging
import os
import threading
import time
from datetime import datetime, timedelta, timezone
from typing import Annotated

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from .features import Point
from .geo import haversine_m
from .model import ANOMALY_THRESHOLD, BaselineModel, Model, ModelStore

LOG = logging.getLogger("eta_ml.api")

MAX_TRACK_POINTS = 1000
#: Straight-line distance understates road distance; this is the usual correction for mixed roads.
DEFAULT_ROAD_FACTOR = 1.3
VehicleType = Annotated[str, Field(pattern="^(truck|van|car|bike|trailer)$")]


class LatLon(BaseModel):
    lat: float = Field(ge=-90, le=90)
    lon: float = Field(ge=-180, le=180)


class EtaRequest(BaseModel):
    origin: LatLon = Field(alias="from")
    destination: LatLon = Field(alias="to")
    depart_at: datetime | None = Field(default=None, alias="departAt")
    vehicle_type: VehicleType = Field(default="truck", alias="vehicleType")
    speed_limit_kph: float | None = Field(default=None, ge=5, le=200, alias="speedLimitKph")

    model_config = {"populate_by_name": True}


class TrackPoint(BaseModel):
    recorded_at: datetime = Field(alias="recordedAt")
    lat: float = Field(ge=-90, le=90)
    lon: float = Field(ge=-180, le=180)
    speed_kph: float = Field(default=0, ge=0, le=400, alias="speedKph")
    heading_deg: float = Field(default=0, ge=0, le=360, alias="headingDeg")

    model_config = {"populate_by_name": True}


class AnomalyRequest(BaseModel):
    points: list[TrackPoint] = Field(min_length=2, max_length=MAX_TRACK_POINTS)
    threshold: float = Field(default=ANOMALY_THRESHOLD, ge=0, le=1)


class ModelHolder:
    """The live model, swapped atomically, reloaded when the pointer file changes."""

    def __init__(self, store: ModelStore, check_every_s: float = 30.0):
        self.store = store
        self.check_every_s = check_every_s
        self._lock = threading.Lock()
        self._model: Model = BaselineModel()
        self._seen_mtime: float | None = None
        self._last_check = 0.0
        self.reload()

    def reload(self) -> Model:
        with self._lock:
            mtime = self.store.pointer_mtime()
            try:
                loaded = self.store.load_current()
            except Exception:
                LOG.exception("could not load the published model; keeping the current one")
                return self._model
            self._seen_mtime = mtime
            self._model = loaded if loaded is not None else BaselineModel()
            LOG.info("serving model %s %s", self._model.info.name, self._model.info.version)
            return self._model

    def current(self) -> Model:
        now = time.monotonic()
        if now - self._last_check > self.check_every_s:
            self._last_check = now
            if self.store.pointer_mtime() != self._seen_mtime:
                self.reload()
        return self._model


def create_app(store: ModelStore | None = None) -> FastAPI:
    store = store or ModelStore(os.environ.get("MODEL_DIR", "./models"))
    holder = ModelHolder(store)
    road_factor = float(os.environ.get("ROAD_DISTANCE_FACTOR", DEFAULT_ROAD_FACTOR))
    app = FastAPI(title="Trackline eta-ml", version="0.1.0")

    @app.get("/healthz")
    def healthz():
        info = holder.current().info
        return {"ready": True, "model": info.name, "version": info.version}

    @app.get("/v1/model")
    def model_info():
        return holder.current().info

    @app.post("/v1/model/reload")
    def reload_model():
        return holder.reload().info

    @app.post("/v1/eta")
    def eta(req: EtaRequest):
        model = holder.current()
        depart = req.depart_at or datetime.now(timezone.utc)
        if depart.tzinfo is None:
            depart = depart.replace(tzinfo=timezone.utc)

        straight_m = haversine_m(req.origin.lat, req.origin.lon, req.destination.lat, req.destination.lon)
        distance_km = straight_m * road_factor / 1000
        speed = model.predict_speed_kph(depart, req.vehicle_type, req.speed_limit_kph)
        seconds = int(round(distance_km / speed * 3600))
        return {
            "distanceKm": round(distance_km, 3),
            "predictedSpeedKph": round(speed, 1),
            "etaSeconds": seconds,
            "departAt": depart.isoformat(),
            "arrivalAt": (depart + timedelta(seconds=seconds)).isoformat(),
            "model": model.info,
        }

    @app.post("/v1/anomaly/score")
    def anomaly(req: AnomalyRequest):
        model = holder.current()
        ordered = sorted(req.points, key=lambda p: p.recorded_at)
        if len({p.recorded_at for p in ordered}) != len(ordered):
            raise HTTPException(status_code=422, detail="points must have distinct recordedAt values")

        track = [Point(p.recorded_at.timestamp(), p.lat, p.lon, p.speed_kph, p.heading_deg) for p in ordered]
        scores = model.anomaly_scores(track)
        steps = [
            {"recordedAt": p.recorded_at.isoformat(), "score": round(s, 4), "anomalous": s >= req.threshold}
            for p, s in zip(ordered[1:], scores)
        ]
        return {
            "maxScore": round(max(scores), 4) if scores else 0.0,
            "anomalousSteps": sum(1 for s in steps if s["anomalous"]),
            "threshold": req.threshold,
            "steps": steps,
            "model": model.info,
        }

    return app
