// Drivers (openapi.yaml listDrivers).
import { Controller, Get, Inject, Injectable, Module, Query, Req, UseGuards } from '@nestjs/common';
import { Request } from 'express';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { PageQuery, RolesGuard } from '../common/http';
import { TenantGuard } from '../common/tenant-guard';

@Injectable()
export class DriversService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(tenantId: string, page: number, pageSize: number) {
    const offset = (page - 1) * pageSize;
    const { rows } = await this.pool.query(
      `SELECT id, full_name AS "fullName", licence_number AS "licenceNumber", phone, email, status
       FROM driver WHERE tenant_id = $1
       ORDER BY full_name LIMIT $2 OFFSET $3`,
      [tenantId, pageSize, offset],
    );
    const { rows: countRows } = await this.pool.query(
      `SELECT count(*)::int AS total FROM driver WHERE tenant_id = $1`,
      [tenantId],
    );
    return { data: rows, page, pageSize, total: countRows[0].total };
  }
}

@Controller('v1/drivers')
@UseGuards(TenantGuard, RolesGuard)
export class DriversController {
  constructor(private readonly drivers: DriversService) {}

  @Get()
  list(@Req() req: Request, @Query() query: PageQuery) {
    return this.drivers.list(req.tenant!.tenantId, query.page, query.pageSize);
  }
}

@Module({
  controllers: [DriversController],
  providers: [DriversService],
})
export class DriversModule {}
