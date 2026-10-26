'use client';

// Vehicle list: search, filter, sort and page through the tenant's vehicles
// (GET /v1/vehicles). Each row links to the vehicle's route history.
import { useEffect, useState } from 'react';
import { api, ApiError, Fleet, Vehicle, VehicleQuery } from '../../lib/api-client';

const PAGE_SIZE = 25;

export default function VehiclesPage() {
  const [fleets, setFleets] = useState<Fleet[]>([]);
  const [rows, setRows] = useState<Vehicle[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<'' | Vehicle['status']>('');
  const [fleetId, setFleetId] = useState('');
  const [sort, setSort] = useState<NonNullable<VehicleQuery['sort']>>('updatedAt');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // /vehicles?fleetId=... arrives from the dashboard.
  useEffect(() => {
    const initial = new URLSearchParams(window.location.search).get('fleetId');
    if (initial) setFleetId(initial);
    api.listFleets().then((p) => setFleets(p.data)).catch(() => undefined);
  }, []);

  // Wait for typing to pause before querying.
  useEffect(() => {
    const t = setTimeout(() => {
      setQ(search);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    let current = true;
    setLoading(true);
    api
      .listVehicles({ q, status: status || undefined, fleetId: fleetId || undefined, sort, page, pageSize: PAGE_SIZE })
      .then((res) => {
        if (!current) return;
        setRows(res.data);
        setTotal(res.total);
        setError(null);
      })
      .catch((e: unknown) => current && setError(e instanceof ApiError ? e.problem.title : 'Failed to load vehicles'))
      .finally(() => current && setLoading(false));
    return () => {
      current = false;
    };
  }, [q, status, fleetId, sort, page]);

  const fleetName = (id: string) => fleets.find((f) => f.id === id)?.name ?? id.slice(0, 8);
  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section>
      <h1>Vehicles</h1>

      <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
        <input
          type="search"
          placeholder="Registration or reference"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          maxLength={64}
        />
        <select
          value={fleetId}
          onChange={(e) => {
            setFleetId(e.target.value);
            setPage(1);
          }}
        >
          <option value="">All fleets</option>
          {fleets.map((f) => (
            <option key={f.id} value={f.id}>
              {f.name}
            </option>
          ))}
        </select>
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value as typeof status);
            setPage(1);
          }}
        >
          <option value="">Any status</option>
          <option value="active">Active</option>
          <option value="maintenance">Maintenance</option>
          <option value="retired">Retired</option>
        </select>
        <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)}>
          <option value="updatedAt">Recently updated</option>
          <option value="registration">Registration</option>
          <option value="externalRef">Reference</option>
        </select>
      </div>

      {error && <p role="alert">Error: {error}</p>}
      {loading && rows.length === 0 && <p>Loading vehicles…</p>}
      {!loading && rows.length === 0 && !error && <p>No vehicles match.</p>}

      {rows.length > 0 && (
        <table style={{ borderCollapse: 'collapse', width: '100%' }}>
          <thead>
            <tr>
              {['Registration', 'Reference', 'Fleet', 'Type', 'Status', 'Device', 'Last seen'].map((h) => (
                <th key={h} align="left" style={{ padding: '0.25rem 0.5rem', borderBottom: '1px solid #ddd' }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((v) => (
              <tr key={v.id}>
                <td style={{ padding: '0.25rem 0.5rem' }}>
                  <a href={`/vehicles/${v.id}`}>{v.registration}</a>
                </td>
                <td style={{ padding: '0.25rem 0.5rem' }}>{v.externalRef}</td>
                <td style={{ padding: '0.25rem 0.5rem' }}>{fleetName(v.fleetId)}</td>
                <td style={{ padding: '0.25rem 0.5rem' }}>{v.vehicleType}</td>
                <td style={{ padding: '0.25rem 0.5rem' }}>{v.status}</td>
                <td style={{ padding: '0.25rem 0.5rem' }}>{v.device?.status ?? '—'}</td>
                <td style={{ padding: '0.25rem 0.5rem' }}>
                  {v.device?.lastSeenAt ? new Date(v.device.lastSeenAt).toLocaleString() : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <p style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
        <button disabled={page <= 1} onClick={() => setPage(page - 1)}>
          Previous
        </button>
        <span>
          Page {page} of {lastPage} · {total} vehicles
        </span>
        <button disabled={page >= lastPage} onClick={() => setPage(page + 1)}>
          Next
        </button>
      </p>
    </section>
  );
}
