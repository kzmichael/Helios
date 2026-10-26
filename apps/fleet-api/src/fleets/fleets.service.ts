// Fleet and vehicle CRUD. Implements the corresponding paths in
// apps/_contracts/openapi.yaml. Every query is scoped by tenantId from
// TenantContext — never trust an id in the URL alone.
import { BadRequestException, ConflictException, Inject, Injectable, NotFoundException } from '@nestjs/common';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { TenantsService } from '../tenants/tenants.service';
import {
  FleetInput,
  FleetPatch,
  HistoryQuery,
  VehicleInput,
  VehicleListQuery,
  VehiclePatch,
} from './dto';

export { FleetInput, VehicleInput };

const FLEET_COLUMNS = `f.id, f.name, f.description, f.depot_lat AS "depotLat", f.depot_lon AS "depotLon",
       (SELECT count(*)::int FROM vehicle v WHERE v.fleet_id = f.id) AS "vehicleCount",
       f.created_at AS "createdAt", f.updated_at AS "updatedAt"`;

const VEHICLE_COLUMNS = `v.id, v.fleet_id AS "fleetId", v.external_ref AS "externalRef", v.registration,
       v.make, v.model, v.year, v.vehicle_type AS "vehicleType", v.status,
       v.speed_limit_kph AS "speedLimitKph",
       (SELECT json_build_object('id', d.id, 'serial', d.serial, 'firmware', d.firmware,
                                 'status', d.status, 'lastSeenAt', d.last_seen_at)
        FROM device d WHERE d.vehicle_id = v.id AND d.status <> 'decommissioned') AS device,
       v.created_at AS "createdAt", v.updated_at AS "updatedAt"`;

const VEHICLE_SORTS: Record<string, string> = {
  updatedAt: 'v.updated_at DESC',
  registration: 'v.registration ASC',
  externalRef: 'v.external_ref ASC',
};

// PATCH bodies are applied by whitelisted column, never by key name from the caller.
const FLEET_PATCH_COLUMNS: Record<string, string> = {
  name: 'name',
  description: 'description',
  depotLat: 'depot_lat',
  depotLon: 'depot_lon',
};

const VEHICLE_PATCH_COLUMNS: Record<string, string> = {
  fleetId: 'fleet_id',
  externalRef: 'external_ref',
  registration: 'registration',
  make: 'make',
  model: 'model',
  year: 'year',
  vehicleType: 'vehicle_type',
  status: 'status',
  speedLimitKph: 'speed_limit_kph',
};

function buildSet(patch: object, columns: Record<string, string>, firstParam: number) {
  const sets: string[] = [];
  const values: unknown[] = [];
  for (const [key, column] of Object.entries(columns)) {
    const v = (patch as Record<string, unknown>)[key];
    if (v === undefined) continue;
    values.push(v);
    sets.push(`${column} = $${firstParam + values.length - 1}`);
  }
  return { sets, values };
}

/** A fleet id from the request body must belong to the caller's tenant, or it names nothing. */
export async function assertFleetInTenant(db: { query: Pool['query'] }, tenantId: string, fleetId: string) {
  const { rows } = await db.query(`SELECT 1 FROM fleet WHERE tenant_id = $1 AND id = $2`, [tenantId, fleetId]);
  if (!rows[0]) throw new NotFoundException({ detail: 'Fleet not found.' });
}

@Injectable()
export class FleetsService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(tenantId: string, page: number, pageSize: number) {
    const offset = (page - 1) * pageSize;
    const { rows } = await this.pool.query(
      `SELECT ${FLEET_COLUMNS}
       FROM fleet f WHERE f.tenant_id = $1
       ORDER BY f.created_at DESC LIMIT $2 OFFSET $3`,
      [tenantId, pageSize, offset],
    );
    const { rows: countRows } = await this.pool.query(
      `SELECT count(*)::int AS total FROM fleet WHERE tenant_id = $1`,
      [tenantId],
    );
    return { data: rows, page, pageSize, total: countRows[0].total };
  }

  async create(tenantId: string, input: FleetInput) {
    const { rows } = await this.pool.query(
      `INSERT INTO fleet (tenant_id, name, description, depot_lat, depot_lon)
       VALUES ($1, $2, $3, $4, $5)
       RETURNING id`,
      [tenantId, input.name, input.description ?? null, input.depotLat ?? null, input.depotLon ?? null],
    );
    return this.getOrThrow(tenantId, rows[0].id);
  }

  // Cross-tenant access must return 404, never 403 (auth.md §4) — the WHERE
  // clause below achieves this for free: a row belonging to another tenant is
  // indistinguishable from a row that does not exist.
  async getOrThrow(tenantId: string, fleetId: string) {
    const { rows } = await this.pool.query(
      `SELECT ${FLEET_COLUMNS}
       FROM fleet f WHERE f.tenant_id = $1 AND f.id = $2`,
      [tenantId, fleetId],
    );
    if (!rows[0]) throw new NotFoundException();
    return rows[0];
  }

  async update(tenantId: string, fleetId: string, patch: FleetPatch) {
    const { sets, values } = buildSet(patch, FLEET_PATCH_COLUMNS, 3);
    if (sets.length > 0) {
      const { rowCount } = await this.pool.query(
        `UPDATE fleet SET ${sets.join(', ')}, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
        [tenantId, fleetId, ...values],
      );
      if (!rowCount) throw new NotFoundException();
    }
    return this.getOrThrow(tenantId, fleetId);
  }

  /** A fleet that still has vehicles cannot be deleted; deleting it would delete them. */
  async remove(tenantId: string, fleetId: string): Promise<void> {
    await this.getOrThrow(tenantId, fleetId);
    const { rows } = await this.pool.query(
      `SELECT count(*)::int AS n FROM vehicle WHERE tenant_id = $1 AND fleet_id = $2`,
      [tenantId, fleetId],
    );
    if (rows[0].n > 0) {
      throw new ConflictException({
        detail: `The fleet still has ${rows[0].n} vehicle(s); move or retire them first.`,
      });
    }
    await this.pool.query(`DELETE FROM fleet WHERE tenant_id = $1 AND id = $2`, [tenantId, fleetId]);
  }
}

@Injectable()
export class VehiclesService {
  constructor(
    @Inject(PG_POOL) private readonly pool: Pool,
    private readonly tenants: TenantsService,
  ) {}

  async list(tenantId: string, query: VehicleListQuery) {
    const { page, pageSize } = query;
    const offset = (page - 1) * pageSize;
    const params: unknown[] = [tenantId];
    let where = 'v.tenant_id = $1';
    if (query.fleetId) {
      params.push(query.fleetId);
      where += ` AND v.fleet_id = $${params.length}`;
    }
    if (query.status) {
      params.push(query.status);
      where += ` AND v.status = $${params.length}`;
    }
    if (query.q) {
      // Escape LIKE wildcards so "q" is matched literally.
      params.push(`%${query.q.replace(/[\\%_]/g, (c) => `\\${c}`)}%`);
      where += ` AND (v.registration ILIKE $${params.length} OR v.external_ref ILIKE $${params.length})`;
    }
    const filterParams = [...params];
    params.push(pageSize, offset);

    const { rows } = await this.pool.query(
      `SELECT ${VEHICLE_COLUMNS}
       FROM vehicle v WHERE ${where}
       ORDER BY ${VEHICLE_SORTS[query.sort] ?? VEHICLE_SORTS.updatedAt}
       LIMIT $${params.length - 1} OFFSET $${params.length}`,
      params,
    );
    const { rows: countRows } = await this.pool.query(
      `SELECT count(*)::int AS total FROM vehicle v WHERE ${where}`,
      filterParams,
    );
    return { data: rows, page, pageSize, total: countRows[0].total };
  }

  /**
   * Creating a vehicle is where the device quota from limits.md §3 is checked —
   * inside one transaction so the count-then-insert can't race under load.
   */
  async create(tenantId: string, input: VehicleInput) {
    const client = await this.pool.connect();
    try {
      await client.query('BEGIN');
      await this.tenants.assertDeviceQuotaNotExceeded(client, tenantId);
      await assertFleetInTenant(client, tenantId, input.fleetId);

      const { rows } = await client.query(
        `INSERT INTO vehicle (tenant_id, fleet_id, external_ref, registration, make, model,
                               year, vehicle_type, status, speed_limit_kph)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
         RETURNING id`,
        [
          tenantId,
          input.fleetId,
          input.externalRef,
          input.registration,
          input.make ?? null,
          input.model ?? null,
          input.year ?? null,
          input.vehicleType ?? 'truck',
          input.status ?? 'active',
          input.speedLimitKph ?? 80.0,
        ],
      );
      await client.query('COMMIT');
      return this.getOrThrow(tenantId, rows[0].id);
    } catch (err) {
      await client.query('ROLLBACK');
      throw err;
    } finally {
      client.release();
    }
  }

  async getOrThrow(tenantId: string, vehicleId: string) {
    const { rows } = await this.pool.query(
      `SELECT ${VEHICLE_COLUMNS}
       FROM vehicle v WHERE v.tenant_id = $1 AND v.id = $2`,
      [tenantId, vehicleId],
    );
    if (!rows[0]) throw new NotFoundException();
    return rows[0];
  }

  async update(tenantId: string, vehicleId: string, patch: VehiclePatch) {
    if (patch.fleetId) await assertFleetInTenant(this.pool, tenantId, patch.fleetId);

    const { sets, values } = buildSet(patch, VEHICLE_PATCH_COLUMNS, 3);
    if (sets.length > 0) {
      const { rowCount } = await this.pool.query(
        `UPDATE vehicle SET ${sets.join(', ')}, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
        [tenantId, vehicleId, ...values],
      );
      if (!rowCount) throw new NotFoundException();
    }
    return this.getOrThrow(tenantId, vehicleId);
  }

  /**
   * Position history, oldest first. The window is clipped to the tier's retention
   * (limits.md §2): a Free tenant cannot read back further than 7 days.
   */
  async history(tenantId: string, vehicleId: string, query: HistoryQuery) {
    await this.getOrThrow(tenantId, vehicleId);

    const from = new Date(query.from);
    const to = new Date(query.to);
    if (!(from < to)) throw new BadRequestException({ detail: '"from" must be earlier than "to".' });

    const tenant = await this.tenants.get(tenantId);
    const earliest = new Date(Date.now() - (tenant?.retentionDays ?? 7) * 86_400_000);
    const effectiveFrom = from > earliest ? from : earliest;
    if (effectiveFrom >= to) return { data: [] };

    const { rows } = await this.pool.query(
      `SELECT recorded_at AS "recordedAt", lat, lon, speed_kph AS "speedKph", heading_deg AS "headingDeg",
              altitude_m AS "altitudeM", accuracy_m AS "accuracyM", odometer_km AS "odometerKm",
              fuel_level_pct AS "fuelLevelPct", engine_temp_c AS "engineTempC", ignition_on AS "ignitionOn"
       FROM telemetry
       WHERE tenant_id = $1 AND vehicle_id = $2 AND recorded_at >= $3 AND recorded_at <= $4
       ORDER BY recorded_at ASC LIMIT $5`,
      [tenantId, vehicleId, effectiveFrom, to, query.limit],
    );
    return { data: rows };
  }
}
