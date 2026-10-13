'use client';

// Live map: the visible payoff of the "position freshness p99 < 5s" SLO
// (docs/01_APPLICATION.md §5, item 4). Renders a plain list of positions rather
// than an actual map widget for now — plugging in a map library (MapLibre/
// Leaflet) is separate, purely front-end work and not the point of this phase.
import { useEffect, useState } from 'react';
import { connectPositions } from '../../lib/ws-client';
import { api, Position } from '../../lib/api-client';

export default function MapPage() {
  const [positions, setPositions] = useState<Record<string, Position>>({});
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    // Seed from a REST snapshot immediately, then switch to live updates.
    api
      .listPositions()
      .then((res) => {
        const byId: Record<string, Position> = {};
        for (const p of res.data) byId[p.vehicleId] = p;
        setPositions(byId);
      })
      .catch(() => {
        /* handled by the empty-state below */
      });

    const ws = connectPositions(
      (position) => {
        setConnected(true);
        setPositions((prev) => ({ ...prev, [position.vehicleId]: position }));
      },
      () => setConnected(false),
    );

    return () => ws.close();
  }, []);

  const list = Object.values(positions).sort((a, b) => a.vehicleId.localeCompare(b.vehicleId));

  return (
    <section>
      <h1>Live Map</h1>
      <p>
        Realtime channel: <strong>{connected ? 'connected' : 'REST snapshot only'}</strong>{' '}
        <small>(the WebSocket endpoint this expects is not built yet — see lib/ws-client.ts)</small>
      </p>
      <table>
        <thead>
          <tr>
            <th align="left">Vehicle</th>
            <th align="left">Lat</th>
            <th align="left">Lon</th>
            <th align="left">Speed (km/h)</th>
            <th align="left">Ignition</th>
            <th align="left">Recorded at</th>
          </tr>
        </thead>
        <tbody>
          {list.map((p) => (
            <tr key={p.vehicleId} style={{ opacity: p.stale ? 0.5 : 1 }}>
              <td>{p.registration ?? p.vehicleId}</td>
              <td>{p.lat.toFixed(5)}</td>
              <td>{p.lon.toFixed(5)}</td>
              <td>{p.speedKph.toFixed(0)}</td>
              <td>{p.ignitionOn ? 'on' : 'off'}</td>
              <td>{new Date(p.recordedAt).toLocaleTimeString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
