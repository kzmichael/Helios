// Thin REST client. Hand-generated for Phase 0 — a real client generated from
// apps/_contracts/openapi.yaml is the correct long-term shape, but that codegen
// step is not wired yet. Do not add ad-hoc `fetch` calls elsewhere in the app;
// extend this file so every request goes through one place that handles the
// auth header and error shape consistently.
const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8080/v1';

export interface Problem {
  type: string;
  title: string;
  status: number;
  detail?: string;
}

export class ApiError extends Error {
  constructor(public problem: Problem) {
    super(problem.title);
  }
}

/**
 * Reads the access token from wherever the session is held. Phase 0 has no
 * real session/login flow yet — this reads a token set manually via
 * `window.localStorage` for development. Replace with real session handling
 * once an identity provider exists (auth.md §1).
 */
function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return window.localStorage.getItem('trackline_dev_token');
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers ?? {}),
    },
  });

  if (!res.ok) {
    const problem = (await res.json().catch(() => ({
      type: 'about:blank',
      title: res.statusText,
      status: res.status,
    }))) as Problem;
    throw new ApiError(problem);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export interface Page<T> {
  data: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface Fleet {
  id: string;
  name: string;
  description?: string | null;
  vehicleCount?: number;
}

export interface Vehicle {
  id: string;
  fleetId: string;
  registration: string;
  status: 'active' | 'maintenance' | 'retired';
  speedLimitKph: number;
}

export interface Alert {
  id: string;
  vehicleId: string;
  alertType: string;
  severity: 'info' | 'warning' | 'critical';
  summary: string;
  triggeredAt: string;
  acknowledgedAt: string | null;
}

export interface Position {
  vehicleId: string;
  registration?: string;
  lat: number;
  lon: number;
  speedKph: number;
  headingDeg: number;
  ignitionOn: boolean;
  recordedAt: string;
  stale: boolean;
}

export const api = {
  listFleets: () => request<Page<Fleet>>('/fleets'),
  listVehicles: (fleetId?: string) =>
    request<Page<Vehicle>>(`/vehicles${fleetId ? `?fleetId=${fleetId}` : ''}`),
  listAlerts: (params: { severity?: string; acknowledged?: boolean } = {}) => {
    const qs = new URLSearchParams();
    if (params.severity) qs.set('severity', params.severity);
    if (params.acknowledged !== undefined) qs.set('acknowledged', String(params.acknowledged));
    const query = qs.toString();
    return request<Page<Alert>>(`/alerts${query ? `?${query}` : ''}`);
  },
  acknowledgeAlert: (alertId: string) =>
    request<Alert>(`/alerts/${alertId}/acknowledge`, { method: 'POST' }),
  listPositions: () => request<{ data: Position[]; generatedAt: string }>('/positions'),
};
