'use client';

// Alert feed + acknowledge action. Wires to fleet-api's /alerts endpoints per
// apps/_contracts/openapi.yaml.
import { useEffect, useState } from 'react';
import { api, Alert, ApiError } from '../../lib/api-client';

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    api
      .listAlerts({ acknowledged: false })
      .then((page) => setAlerts(page.data))
      .catch((e: unknown) => setError(e instanceof ApiError ? e.problem.title : 'Failed to load alerts'));
  };

  useEffect(load, []);

  const acknowledge = async (id: string) => {
    try {
      await api.acknowledgeAlert(id);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.problem.title : 'Failed to acknowledge alert');
    }
  };

  return (
    <section>
      <h1>Open Alerts</h1>
      {error && <p role="alert">Error: {error}</p>}
      {alerts.length === 0 && <p>No open alerts.</p>}
      <ul>
        {alerts.map((a) => (
          <li key={a.id} style={{ marginBottom: '0.5rem' }}>
            <span style={{ fontWeight: 'bold', textTransform: 'uppercase' }}>{a.severity}</span>{' '}
            {a.summary} <small>({new Date(a.triggeredAt).toLocaleString()})</small>{' '}
            <button onClick={() => acknowledge(a.id)}>Acknowledge</button>
          </li>
        ))}
      </ul>
    </section>
  );
}
