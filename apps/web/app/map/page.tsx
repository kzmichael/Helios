'use client';

// Live map: the visible payoff of the "position freshness p99 < 5s" SLO
// (docs/01_APPLICATION.md §5, item 4). First paint comes from the REST snapshot
// (GET /v1/positions); after that every position arrives over the WebSocket.
import { useEffect, useRef, useState } from 'react';
import { LiveMap, LiveMapHandle } from '../../components/live-map';
import { connectPositions } from '../../lib/ws-client';
import { api, ApiError, Geofence } from '../../lib/api-client';

export default function MapPage() {
  const map = useRef<LiveMapHandle>(null);
  const [connected, setConnected] = useState(false);
  const [vehicles, setVehicles] = useState(0);
  const [geofences, setGeofences] = useState<Geofence[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const snapshot = () =>
      api
        .listPositions()
        .then((res) => {
          map.current?.setAll(res.data);
          setError(null);
        })
        .catch((e: unknown) => setError(e instanceof ApiError ? e.problem.title : 'Failed to load positions'));

    snapshot();
    api.listGeofences().then((res) => setGeofences(res.data)).catch(() => undefined);

    // Reconnecting refetches the snapshot: frames sent while disconnected are not replayed.
    const ws = connectPositions(
      (position) => map.current?.upsert(position),
      (up) => {
        setConnected(up);
        if (up) snapshot();
      },
    );

    const tick = setInterval(() => {
      map.current?.refreshStyles();
      setVehicles(map.current?.size() ?? 0);
    }, 1000);

    return () => {
      ws.close();
      clearInterval(tick);
    };
  }, []);

  return (
    <section>
      <h1>Live Map</h1>
      {error && <p role="alert">Error: {error}</p>}
      <p>
        Realtime channel: <strong>{connected ? 'connected' : 'REST snapshot only'}</strong> · {vehicles} vehicles ·
        grey = no report for 5 minutes, dark grey = ignition off
      </p>
      <LiveMap ref={map} geofences={geofences} />
    </section>
  );
}
