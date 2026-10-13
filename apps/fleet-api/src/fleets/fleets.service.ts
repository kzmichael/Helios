// Fleet and vehicle CRUD. Implements the corresponding paths in
// apps/_contracts/openapi.yaml. Every query is scoped by tenantId from
// TenantContext — never trust an id in the URL alone.
import { Inject, Injectable, NotFoundException } from '@nestjs/common';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { TenantsService } from '../tenants/tenants.service';

export interface FleetInput {
  name: string;
  description?: string;
  depotLat?: number;
  depotLon?: number;
}

export interface VehicleInput {
  fleetId: string;
  externalRef: string;
  registration: string;
  make?: string;
  model?: string;
  year?: number;
  vehicleType?: string;
  speedLimitKph?: number;
}

@Injectable()
export class FleetsService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(tenantId: string, page: number, pageSize: number) {
    const offset = (page - 1) * pageSize;
    const { rows } = await this.pool.query(
      `SELECT id, name, description, depot_lat AS "depotLat", depot_lon AS "depotLon",
              created_at AS "createdAt", updated_at AS "updatedAt"
       FROM fleet WHERE tenant_id = $1
       ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
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
       RETURNING id, name, description, depot_lat AS "depotLat", depot_lon AS "depotLon",
                 created_at AS "createdAt", updated_at AS "updatedAt"`,
      [tenantId, input.name, input.description ?? null, input.depotLat ?? null, input.depotLon ?? null],
    );
    return rows[0];
  }

  // Cross-tenant access must return 404, never 403 (auth.md §4) — the WHERE
  // clause below achieves this for free: a row belonging to another tenant is
  // indistinguishable from a row that does not exist.
  async getOrThrow(tenantId: string, fleetId: string) {
    const { rows } = await this.pool.query(
      `SELECT id, name, description, depot_lat AS "depotLat", depot_lon AS "depotLon",
              created_at AS "createdAt", updated_at AS "updatedAt"
       FROM fleet WHERE tenant_id = $1 AND id = $2`,
      [tenantId, fleetId],
    );
    if (!rows[0]) throw new NotFoundException();
    return rows[0];
  }
}

@Injectable()
export class VehiclesService {
  constructor(
    @Inject(PG_POOL) private readonly pool: Pool,
    private readonly tenants: TenantsService,
  ) {}

  async list(tenantId: string, page: number, pageSize: number, fleetId?: string) {
    const offset = (page - 1) * pageSize;
    const params: unknown[] = [tenantId];
    let where = 'tenant_id = $1';
    if (fleetId) {
      params.push(fleetId);
      where += ` AND fleet_id = $${params.length}`;
    }
    params.push(pageSize, offset);

    const { rows } = await this.pool.query(
      `SELECT id, fleet_id AS "fleetId", external_ref AS "externalRef", registration,
              make, model, year, vehicle_type AS "vehicleType", status,
              speed_limit_kph AS "speedLimitKph",
              created_at AS "createdAt", updated_at AS "updatedAt"
       FROM vehicle WHERE ${where}
       ORDER BY updated_at DESC LIMIT $${params.length - 1} OFFSET $${params.length}`,
      params,
    );
    return { data: rows, page, pageSize };
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

      const { rows } = await client.query(
        `INSERT INTO vehicle (tenant_id, fleet_id, external_ref, registration, make, model,
                               year, vehicle_type, speed_limit_kph)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
         RETURNING id, fleet_id AS "fleetId", external_ref AS "externalRef", registration,
                   make, model, year, vehicle_type AS "vehicleType", status,
                   speed_limit_kph AS "speedLimitKph",
                   created_at AS "createdAt", updated_at AS "updatedAt"`,
        [
          tenantId,
          input.fleetId,
          input.externalRef,
          input.registration,
          input.make ?? null,
          input.model ?? null,
          input.year ?? null,
          input.vehicleType ?? 'truck',
          input.speedLimitKph ?? 80.0,
        ],
      );
      await client.query('COMMIT');
      return rows[0];
    } catch (err) {
      await client.query('ROLLBACK');
      throw err;
    } finally {
      client.release();
    }
  }

  async getOrThrow(tenantId: string, vehicleId: string) {
    const { rows } = await this.pool.query(
      `SELECT id, fleet_id AS "fleetId", external_ref AS "externalRef", registration,
              make, model, year, vehicle_type AS "vehicleType", status,
              speed_limit_kph AS "speedLimitKph",
              created_at AS "createdAt", updated_at AS "updatedAt"
       FROM vehicle WHERE tenant_id = $1 AND id = $2`,
      [tenantId, vehicleId],
    );
    if (!rows[0]) throw new NotFoundException();
    return rows[0];
  }
}
