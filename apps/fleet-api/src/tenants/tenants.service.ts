// Tenant module: quota enforcement per apps/_contracts/limits.md §3.
//
// Device-count quota is checked HERE, transactionally, because only fleet-api
// (with a real database transaction) can count safely under concurrent writes.
// edge-gateway's rate limiter answers a different question — "too fast right
// now?" — not "entitled at all?".
import { Inject, Injectable, Module, UnprocessableEntityException } from '@nestjs/common';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';

export interface Tenant {
  id: string;
  slug: string;
  name: string;
  tier: 'free' | 'pro' | 'enterprise';
  maxDevices: number;
  maxHz: number;
}

@Injectable()
export class TenantsService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async get(tenantId: string): Promise<Tenant | null> {
    const { rows } = await this.pool.query(
      `SELECT id, slug, name, tier, max_devices AS "maxDevices", max_hz AS "maxHz"
       FROM tenant WHERE id = $1 AND status = 'active'`,
      [tenantId],
    );
    return rows[0] ?? null;
  }

  /**
   * Enforces the device quota from limits.md §2/§3 before a new device may be
   * provisioned. Runs inside the caller's transaction so the count-then-insert
   * is atomic under concurrent requests — the whole reason this lives in
   * fleet-api and not at the edge.
   */
  async assertDeviceQuotaNotExceeded(client: { query: Pool['query'] }, tenantId: string): Promise<void> {
    const tenantRes = await client.query(
      `SELECT max_devices AS "maxDevices" FROM tenant WHERE id = $1 FOR UPDATE`,
      [tenantId],
    );
    const tenant = tenantRes.rows[0];
    if (!tenant) {
      throw new UnprocessableEntityException({
        type: 'https://trackline.local/errors/unknown-tenant',
        title: 'Unknown tenant',
        status: 422,
      });
    }

    const countRes = await client.query(
      `SELECT count(*)::int AS count FROM device WHERE tenant_id = $1 AND status <> 'decommissioned'`,
      [tenantId],
    );
    const current = countRes.rows[0].count as number;

    if (current >= tenant.maxDevices) {
      throw new UnprocessableEntityException({
        type: 'https://trackline.local/errors/quota-exceeded',
        title: 'Device quota exceeded',
        status: 422,
        detail: `Tier permits ${tenant.maxDevices} devices; ${current} are in use.`,
      });
    }
  }
}

@Module({
  providers: [TenantsService],
  exports: [TenantsService],
})
export class TenantsModule {}
