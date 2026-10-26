from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest
from fastapi.testclient import TestClient

from eta_ml.api import create_app
from eta_ml.model import ModelStore

from .test_models import synthetic_steps, synthetic_windows
from eta_ml import train


@pytest.fixture()
def client(tmp_path):
    return TestClient(create_app(ModelStore(tmp_path))), tmp_path


def test_health_reports_the_baseline_when_nothing_is_trained(client):
    c, _ = client
    body = c.get("/healthz").json()
    assert body == {"ready": True, "model": "baseline", "version": "rules-1"}


def test_eta_for_a_known_leg(client):
    c, _ = client
    r = c.post(
        "/v1/eta",
        json={
            "from": {"lat": 19.0760, "lon": 72.8777},
            "to": {"lat": 18.5204, "lon": 73.8567},
            "departAt": "2026-09-09T05:00:00Z",
            "vehicleType": "truck",
            "speedLimitKph": 60,
        },
    )
    assert r.status_code == 200
    body = r.json()
    assert 145 < body["distanceKm"] < 165  # ~119 km straight line x 1.3
    assert body["predictedSpeedKph"] == 36.0  # baseline: 60% of the limit
    assert abs(body["etaSeconds"] - body["distanceKm"] / 36 * 3600) < 2
    assert body["model"]["name"] == "baseline"
    arrival = datetime.fromisoformat(body["arrivalAt"])
    assert arrival - datetime(2026, 9, 9, 5, tzinfo=timezone.utc) == timedelta(seconds=body["etaSeconds"])


def test_eta_rejects_bad_input(client):
    c, _ = client
    ok = {"from": {"lat": 19, "lon": 72}, "to": {"lat": 18, "lon": 73}}
    assert c.post("/v1/eta", json={**ok, "from": {"lat": 91, "lon": 0}}).status_code == 422
    assert c.post("/v1/eta", json={**ok, "vehicleType": "spaceship"}).status_code == 422
    assert c.post("/v1/eta", json={**ok, "speedLimitKph": 0}).status_code == 422
    assert c.post("/v1/eta", json={"from": ok["from"]}).status_code == 422


def test_anomaly_endpoint_flags_the_jump(client):
    c, _ = client
    base = datetime(2026, 9, 9, 5, tzinfo=timezone.utc)
    pts = []
    lat = 19.0
    for i in range(12):
        lat += 0.5 if i == 6 else 0.0005
        pts.append({"recordedAt": (base + timedelta(seconds=5 * i)).isoformat(), "lat": lat, "lon": 72.8, "speedKph": 40})
    body = c.post("/v1/anomaly/score", json={"points": pts}).json()

    assert body["anomalousSteps"] >= 1
    assert body["maxScore"] == 1.0
    assert len(body["steps"]) == 11


def test_anomaly_endpoint_limits_and_validation(client):
    c, _ = client
    one = {"recordedAt": "2026-09-09T05:00:00Z", "lat": 19, "lon": 72}
    assert c.post("/v1/anomaly/score", json={"points": [one]}).status_code == 422
    assert c.post("/v1/anomaly/score", json={"points": [one, one]}).status_code == 422  # same timestamp
    many = [{"recordedAt": (datetime(2026, 9, 9, tzinfo=timezone.utc) + timedelta(seconds=i)).isoformat(), "lat": 19, "lon": 72} for i in range(1001)]
    assert c.post("/v1/anomaly/score", json={"points": many}).status_code == 422


def test_publishing_a_model_is_picked_up_by_reload_without_a_restart(client):
    c, tmp = client
    assert c.get("/v1/model").json()["name"] == "baseline"

    ModelStore(tmp).publish(train.fit(synthetic_windows(1200), synthetic_steps(), min_rows=500))
    reloaded = c.post("/v1/model/reload").json()

    assert reloaded["name"] == "hgb-eta+iforest"
    assert c.get("/healthz").json()["model"] == "hgb-eta+iforest"
    eta = c.post("/v1/eta", json={"from": {"lat": 19, "lon": 72}, "to": {"lat": 19.2, "lon": 72.2}}).json()
    assert eta["model"]["name"] == "hgb-eta+iforest"
