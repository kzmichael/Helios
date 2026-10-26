'use client';

// A vehicle's route over a time window: the polyline, a start and an end marker.
import 'leaflet/dist/leaflet.css';
import { useEffect, useRef } from 'react';
import type { Map as LeafletMap } from 'leaflet';
import type { TelemetryPoint } from '../lib/api-client';

const TILE_URL = process.env.NEXT_PUBLIC_MAP_TILE_URL ?? 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
const TILE_ATTRIBUTION =
  process.env.NEXT_PUBLIC_MAP_TILE_ATTRIBUTION ??
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

export function RouteMap({ points }: { points: TelemetryPoint[] }) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<LeafletMap | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const L = await import('leaflet');
      if (cancelled || !container.current) return;

      map.current?.remove();
      const m = L.map(container.current).setView([20.5937, 78.9629], 5);
      map.current = m;
      L.tileLayer(TILE_URL, { attribution: TILE_ATTRIBUTION, maxZoom: 19 }).addTo(m);

      if (points.length === 0) return;
      const path = points.map((p) => [p.lat, p.lon] as [number, number]);
      const line = L.polyline(path, { color: '#2563eb', weight: 3 }).addTo(m);
      L.circleMarker(path[0], { radius: 6, color: '#16a34a', fillOpacity: 1 }).bindTooltip('Start').addTo(m);
      L.circleMarker(path[path.length - 1], { radius: 6, color: '#dc2626', fillOpacity: 1 }).bindTooltip('End').addTo(m);
      m.fitBounds(line.getBounds(), { padding: [30, 30] });
    })();
    return () => {
      cancelled = true;
    };
  }, [points]);

  useEffect(
    () => () => {
      map.current?.remove();
      map.current = null;
    },
    [],
  );

  return <div ref={container} style={{ height: '55vh', width: '100%', border: '1px solid #ddd' }} />;
}
