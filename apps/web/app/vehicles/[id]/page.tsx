'use client';

// One vehicle: its details, its recent alerts, and its route history for a chosen
// window (GET /v1/vehicles/{id}/history). The window is clipped server-side to the
// tenant's tier retention, so a Free tenant asking for 30 days gets 7.
import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { RouteMap } from '../../../components/route-map';
import { api, Alert, ApiError, TelemetryPoint, Vehicle } from '../../../lib/api-client';

const WINDOWS = [
  { label: 'Last hour', hours: 1 },
  { label: 'Last 6 hours', hours: 6 },
  { label: 'Last 24 hours', hours: 24 },
  { label: 'Last 7 days', hours: 24 * 7 },
] as const;

/** Great-circle distance in km. */
function haversineKm(a: TelemetryPoint, b: TelemetryPoint): number {
  const rad = (d: number) => (d * Math.PI) / 180;
  const dLat = rad(b.lat - a.lat);
  const dLon = rad(b.lon - a.lon);
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(rad(a.lat)) * Math.cos(rad(b.lat)) * Math.sin(dLon / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(h), Math.sqrt(1 - h));
}

export default function VehiclePage() {
  const { id } = useParams<{ id: string }>();
  const [vehicle, setVehicle] = useState<Vehicle | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [hours, setHours] = useState<number>(6);
  const [points, setPoints] = useState<TelemetryPoint[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getVehicle(id).then(setVehicle).catch((e: unknown) => setError(e instanceof ApiError ? e.problem.title : 'Failed to load vehicle'));
    api.listAlerts({ vehicleId: id, pageSize: 10 }).then((p) => setAlerts(p.data)).catch(() => undefined);
  }, [id]);

  useEffect(() => {
    let current = true;
    setLoadingHistory(true);
    const to = new Date();
    const from = new Date(to.getTime() - hours * 3_600_000);
    api
      .getVehicleHistory(id, from, to, 5000)
      .then((res) => current && setPoints(res.data))
      .catch((e: unknown) => current && setError(e instanceof ApiError ? e.problem.title : 'Failed to load history'))
      .finally(() => current && setLoadingHistory(false));
    return () => {
      current = false;
    };
  }, [id, hours]);

  const stats = useMemo(() => {
    let distanceKm = 0;
    let maxSpeed = 0;
    let speedSum = 0;
    points.forEach((p, i) => {
      if (i > 0) distanceKm += haversineKm(points[i - 1], p);
      maxSpeed = Math.max(maxSpeed, p.speedKph ?? 0);
      speedSum += p.speedKph ?? 0;
    });
    return { distanceKm, maxSpeed, avgSpeed: points.length ? speedSum / points.length : 0 };
  }, [points]);

  if (error && !vehicle) return <p role="alert">Error: {error}</p>;

  return (
    <section>
      <p>
        <a href="/vehicles">← Vehicles</a>
      </p>
      <h1>{vehicle?.registration ?? 'Vehicle'}</h1>
      {vehicle && (
        <p>
          {vehicle.externalRef} · {vehicle.vehicleType} · {vehicle.status} · limit {vehicle.speedLimitKph} km/h
          {vehicle.device && (
            <>
              {' '}
              · device {vehicle.device.serial} ({vehicle.device.status}
              {vehicle.device.lastSeenAt ? `, last seen ${new Date(vehicle.device.lastSeenAt).toLocaleString()}` : ''})
            </>
          )}
        </p>
      )}

      <h2>Route</h2>
      <p style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
        {WINDOWS.map((w) => (
          <button key={w.hours} onClick={() => setHours(w.hours)} disabled={hours === w.hours}>
            {w.label}
          </button>
        ))}
      </p>
      {error && <p role="alert">Error: {error}</p>}
      {loadingHistory ? (
        <p>Loading history…</p>
      ) : points.length === 0 ? (
        <p>No telemetry in this window.</p>
      ) : (
        <p>
          {points.length} points · {stats.distanceKm.toFixed(1)} km · average {stats.avgSpeed.toFixed(0)} km/h · max{' '}
          {stats.maxSpeed.toFixed(0)} km/h
        </p>
      )}
      <RouteMap points={points} />

      <h2>Recent alerts</h2>
      {alerts.length === 0 && <p>No alerts for this vehicle.</p>}
      <ul>
        {alerts.map((a) => (
          <li key={a.id}>
            <strong style={{ textTransform: 'uppercase' }}>{a.severity}</strong> {a.summary}{' '}
            <small>
              ({new Date(a.triggeredAt).toLocaleString()}
              {a.acknowledgedAt ? ', acknowledged' : ''})
            </small>
          </li>
        ))}
      </ul>
    </section>
  );
}
