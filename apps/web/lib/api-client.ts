// Thin REST client. Request and response types are generated from
// apps/_contracts/openapi.yaml (see lib/api-types.ts); the functions themselves
// are hand-written. Do not add ad-hoc `fetch` calls elsewhere in the app;
// extend this file so every request goes through one place that handles the
// auth header and error shape consistently.
import type { components } from './api-types';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8080/v1';
export const DEV_TOKEN_STORAGE_KEY = 'trackline_dev_token';

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
 * Reads the Phase 0 development token. Replace with real session handling once
 * an identity provider exists (auth.md §1).
 */
function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return window.localStorage.getItem(DEV_TOKEN_STORAGE_KEY);
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

// Wire types come from the contract: `pnpm gen:api` regenerates lib/api-types.ts
// from apps/_contracts/openapi.yaml. The generated file is committed because this
// app's Docker build context does not include apps/_contracts.
type Schemas = components['schemas'];

export interface Page<T> {
  data: T[];
  page: number;
  pageSize: number;
  total: number;
}

export type Fleet = Schemas['Fleet'];
export type Vehicle = Schemas['Vehicle'];
export type Alert = Schemas['Alert'];
export type Position = Schemas['Position'];
export type TelemetryPoint = Schemas['TelemetryPoint'];
export type Geofence = Schemas['Geofence'];

export interface VehicleQuery {
  fleetId?: string;
  status?: Vehicle['status'];
  q?: string;
  sort?: 'updatedAt' | 'registration' | 'externalRef';
  page?: number;
  pageSize?: number;
}

function query(params: Record<string, string | number | boolean | undefined>): string {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') qs.set(k, String(v));
  const s = qs.toString();
  return s ? `?${s}` : '';
}

export const api = {
  listFleets: () => request<Page<Fleet>>('/fleets'),
  listVehicles: (params: VehicleQuery = {}) => request<Page<Vehicle>>(`/vehicles${query({ ...params })}`),
  getVehicle: (vehicleId: string) => request<Vehicle>(`/vehicles/${vehicleId}`),
  getVehicleHistory: (vehicleId: string, from: Date, to: Date, limit = 2000) =>
    request<{ data: TelemetryPoint[] }>(
      `/vehicles/${vehicleId}/history${query({ from: from.toISOString(), to: to.toISOString(), limit })}`,
    ),
  listAlerts: (params: { severity?: string; acknowledged?: boolean; vehicleId?: string; pageSize?: number } = {}) =>
    request<Page<Alert>>(`/alerts${query({ ...params })}`),
  acknowledgeAlert: (alertId: string) =>
    request<Alert>(`/alerts/${alertId}/acknowledge`, { method: 'POST' }),
  listPositions: () => request<{ data: Position[]; generatedAt: string }>('/positions'),
  listGeofences: () => request<{ data: Geofence[] }>('/geofences'),
};
