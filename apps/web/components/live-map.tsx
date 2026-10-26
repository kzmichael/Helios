'use client';

// Leaflet map of the tenant's vehicles. Imperative on purpose: a position frame
// can arrive hundreds of times a second for a large fleet, and pushing each one
// through React state would re-render the whole page. The page hands frames to
// this component through the ref and the markers are moved directly.
import 'leaflet/dist/leaflet.css';
import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react';
import type { CircleMarker, LayerGroup, Map as LeafletMap } from 'leaflet';
import type { Geofence, Position } from '../lib/api-client';

const TILE_URL = process.env.NEXT_PUBLIC_MAP_TILE_URL ?? 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
const TILE_ATTRIBUTION =
  process.env.NEXT_PUBLIC_MAP_TILE_ATTRIBUTION ??
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

/** A vehicle that has not reported for this long is drawn as stale. */
const STALE_AFTER_MS = 5 * 60 * 1000;

export interface LiveMapHandle {
  /** Replace everything with a snapshot (first load, and after a reconnect). */
  setAll: (positions: Position[]) => void;
  /** Apply one live frame; an older frame for the same vehicle is ignored. */
  upsert: (position: Position) => void;
  /** Re-evaluate staleness; call periodically. */
  refreshStyles: () => void;
  size: () => number;
}

function colour(p: Position): string {
  if (Date.now() - Date.parse(p.recordedAt) > STALE_AFTER_MS || p.stale) return '#9ca3af';
  return p.ignitionOn ? '#2563eb' : '#6b7280';
}

export const LiveMap = forwardRef<LiveMapHandle, { geofences: Geofence[] }>(function LiveMap({ geofences }, ref) {
  const container = useRef<HTMLDivElement>(null);
  const leaflet = useRef<typeof import('leaflet') | null>(null);
  const map = useRef<LeafletMap | null>(null);
  const fenceLayer = useRef<LayerGroup | null>(null);
  const markers = useRef(new Map<string, CircleMarker>());
  const latest = useRef(new Map<string, Position>());
  const fitted = useRef(false);
  const fencesRef = useRef<Geofence[]>(geofences);

  const draw = (p: Position) => {
    const L = leaflet.current;
    if (!L || !map.current) return;
    const label = `${p.registration ?? p.vehicleId} · ${Math.round(p.speedKph ?? 0)} km/h`;
    let m = markers.current.get(p.vehicleId);
    if (!m) {
      m = L.circleMarker([p.lat, p.lon], { radius: 5, weight: 1, color: '#ffffff', fillOpacity: 0.9 });
      m.on('click', () => {
        window.location.href = `/vehicles/${p.vehicleId}`;
      });
      m.bindTooltip(label);
      m.addTo(map.current);
      markers.current.set(p.vehicleId, m);
    } else {
      m.setLatLng([p.lat, p.lon]);
      m.setTooltipContent(label);
    }
    m.setStyle({ fillColor: colour(p) });
  };

  const fit = () => {
    const L = leaflet.current;
    if (!L || !map.current || fitted.current || latest.current.size === 0) return;
    map.current.fitBounds(L.latLngBounds([...latest.current.values()].map((p) => [p.lat, p.lon] as [number, number])), {
      padding: [30, 30],
      maxZoom: 14,
    });
    fitted.current = true;
  };

  const drawFences = () => {
    const L = leaflet.current;
    if (!L || !fenceLayer.current) return;
    fenceLayer.current.clearLayers();
    for (const g of fencesRef.current) {
      const ring = g.boundary.coordinates[0] as number[][];
      L.polygon(
        ring.map(([lon, lat]) => [lat, lon] as [number, number]),
        { color: g.kind === 'exclusion' ? '#dc2626' : '#16a34a', weight: 1, fillOpacity: 0.08 },
      )
        .bindTooltip(g.name, { sticky: true })
        .addTo(fenceLayer.current);
    }
  };

  useImperativeHandle(ref, () => ({
    setAll(positions) {
      latest.current = new Map(positions.map((p) => [p.vehicleId, p]));
      for (const [id, m] of markers.current) {
        if (!latest.current.has(id)) {
          m.remove();
          markers.current.delete(id);
        }
      }
      for (const p of positions) draw(p);
      fit();
    },
    upsert(p) {
      const cur = latest.current.get(p.vehicleId);
      if (cur && Date.parse(cur.recordedAt) > Date.parse(p.recordedAt)) return;
      latest.current.set(p.vehicleId, p);
      draw(p);
      fit();
    },
    refreshStyles() {
      for (const [id, m] of markers.current) {
        const p = latest.current.get(id);
        if (p) m.setStyle({ fillColor: colour(p) });
      }
    },
    size: () => latest.current.size,
  }));

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const L = await import('leaflet');
      if (cancelled || !container.current) return;
      leaflet.current = L;
      // Canvas rendering keeps a few hundred moving markers cheap.
      map.current = L.map(container.current, { preferCanvas: true }).setView([20.5937, 78.9629], 5);
      L.tileLayer(TILE_URL, { attribution: TILE_ATTRIBUTION, maxZoom: 19 }).addTo(map.current);
      fenceLayer.current = L.layerGroup().addTo(map.current);
      drawFences();
      for (const p of latest.current.values()) draw(p);
      fit();
    })();
    return () => {
      cancelled = true;
      map.current?.remove();
      map.current = null;
      markers.current.clear();
      fitted.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    fencesRef.current = geofences;
    drawFences();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [geofences]);

  return <div ref={container} style={{ height: '70vh', width: '100%', border: '1px solid #ddd' }} />;
});
