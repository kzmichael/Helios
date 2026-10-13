'use client';

// Tenant dashboard shell: fleets, vehicle counts, nothing fancier yet. Wires
// directly to fleet-api's /fleets and /vehicles per apps/_contracts/openapi.yaml.
import { useEffect, useState } from 'react';
import { api, Fleet, ApiError } from '../../lib/api-client';

export default function DashboardPage() {
  const [fleets, setFleets] = useState<Fleet[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .listFleets()
      .then((page) => setFleets(page.data))
      .catch((e: unknown) => setError(e instanceof ApiError ? e.problem.title : 'Failed to load fleets'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p>Loading fleets…</p>;
  if (error) return <p role="alert">Error: {error}</p>;

  return (
    <section>
      <h1>Fleets</h1>
      {fleets.length === 0 && <p>No fleets yet.</p>}
      <ul>
        {fleets.map((f) => (
          <li key={f.id}>
            <strong>{f.name}</strong>
            {f.description ? ` — ${f.description}` : ''}
          </li>
        ))}
      </ul>
    </section>
  );
}
